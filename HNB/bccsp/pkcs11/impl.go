package pkcs11

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"os"

	"HNB/bccsp"
	"HNB/bccsp/sw"
	"HNB/bccsp/utils"
	"HNB/logging"
	"github.com/miekg/pkcs11"
)

var (
	logger           = logging.GetLogIns()
	sessionCacheSize = 10
	LOGTABLE_BCCSP   = "bccsp"
)

func New(opts PKCS11Opts, keyStore bccsp.KeyStore) (bccsp.BCCSP, error) {
	conf := &config{}
	err := conf.setSecurityLevel(opts.SecLevel, opts.HashFamily)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing configuration [%s]", err)
	}

	swCSP, err := sw.New(opts.SecLevel, opts.HashFamily, keyStore)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing fallback SW BCCSP [%s]", err)
	}

	if keyStore == nil {
		return nil, errors.New("Invalid bccsp.KeyStore instance. It must be different from nil.")
	}

	lib := opts.Library
	pin := opts.Pin
	label := opts.Label
	ctx, slot, session, err := loadLib(lib, pin, label)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing PKCS11 library %s %s [%s]",
			lib, label, err)
	}

	sessions := make(chan pkcs11.SessionHandle, sessionCacheSize)
	csp := &impl{swCSP, conf, keyStore, ctx, sessions, slot, lib, opts.Sensitive, opts.SoftVerify}
	csp.returnSession(*session)
	return csp, nil
}

type impl struct {
	bccsp.BCCSP

	conf *config
	ks   bccsp.KeyStore

	ctx      *pkcs11.Ctx
	sessions chan pkcs11.SessionHandle
	slot     uint

	lib          string
	noPrivImport bool
	softVerify   bool
}

func (csp *impl) KeyGen(opts bccsp.KeyGenOpts) (k bccsp.Key, err error) {
	if opts == nil {
		return nil, errors.New("Invalid Opts parameter. It must not be nil.")
	}

	switch opts.(type) {
	case *bccsp.ECDSAKeyGenOpts:
		ski, pub, err := csp.generateECKey(csp.conf.ellipticCurve, opts.Ephemeral())
		if err != nil {
			return nil, fmt.Errorf("Failed generating ECDSA key [%s]", err)
		}
		k = &ecdsaPrivateKey{ski, ecdsaPublicKey{ski, pub}}

	case *bccsp.ECDSAP256KeyGenOpts:
		ski, pub, err := csp.generateECKey(oidNamedCurveP256, opts.Ephemeral())
		if err != nil {
			return nil, fmt.Errorf("Failed generating ECDSA P256 key [%s]", err)
		}

		k = &ecdsaPrivateKey{ski, ecdsaPublicKey{ski, pub}}

	case *bccsp.ECDSAP384KeyGenOpts:
		ski, pub, err := csp.generateECKey(oidNamedCurveP384, opts.Ephemeral())
		if err != nil {
			return nil, fmt.Errorf("Failed generating ECDSA P384 key [%s]", err)
		}

		k = &ecdsaPrivateKey{ski, ecdsaPublicKey{ski, pub}}

	default:
		return csp.BCCSP.KeyGen(opts)
	}

	return k, nil
}

func (csp *impl) KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error) {
	if k == nil {
		return nil, errors.New("Invalid Key. It must not be nil.")
	}

	switch k.(type) {
	case *ecdsaPublicKey:
		if opts == nil {
			return nil, errors.New("Invalid Opts parameter. It must not be nil.")
		}

		ecdsaK := k.(*ecdsaPublicKey)

		switch opts.(type) {

		case *bccsp.ECDSAReRandKeyOpts:
			pubKey := ecdsaK.pub
			if pubKey == nil {
				return nil, errors.New("Public base key cannot be nil.")
			}
			reRandOpts := opts.(*bccsp.ECDSAReRandKeyOpts)
			tempSK := &ecdsa.PublicKey{
				Curve: pubKey.Curve,
				X:     new(big.Int),
				Y:     new(big.Int),
			}

			var k = new(big.Int).SetBytes(reRandOpts.ExpansionValue())
			var one = new(big.Int).SetInt64(1)
			n := new(big.Int).Sub(pubKey.Params().N, one)
			k.Mod(k, n)
			k.Add(k, one)

			tempX, tempY := pubKey.ScalarBaseMult(k.Bytes())
			tempSK.X, tempSK.Y = tempSK.Add(
				pubKey.X, pubKey.Y,
				tempX, tempY,
			)

			isOn := tempSK.Curve.IsOnCurve(tempSK.X, tempSK.Y)
			if !isOn {
				return nil, errors.New("Failed temporary public key IsOnCurve check.")
			}

			ecPt := elliptic.Marshal(tempSK.Curve, tempSK.X, tempSK.Y)
			oid, ok := oidFromNamedCurve(tempSK.Curve)
			if !ok {
				return nil, errors.New("Do not know OID for this Curve.")
			}

			ski, err := csp.importECKey(oid, nil, ecPt, opts.Ephemeral(), publicKeyFlag)
			if err != nil {
				return nil, fmt.Errorf("Failed getting importing EC Public Key [%s]", err)
			}
			reRandomizedKey := &ecdsaPublicKey{ski, tempSK}

			return reRandomizedKey, nil

		default:
			return nil, fmt.Errorf("Unrecognized KeyDerivOpts provided [%s]", opts.Algorithm())

		}
	case *ecdsaPrivateKey:
		if opts == nil {
			return nil, errors.New("Invalid Opts parameter. It must not be nil.")
		}

		ecdsaK := k.(*ecdsaPrivateKey)

		switch opts.(type) {

		case *bccsp.ECDSAReRandKeyOpts:
			reRandOpts := opts.(*bccsp.ECDSAReRandKeyOpts)
			pubKey := ecdsaK.pub.pub
			if pubKey == nil {
				return nil, errors.New("Public base key cannot be nil.")
			}

			secret := csp.getSecretValue(ecdsaK.ski)
			if secret == nil {
				return nil, errors.New("Could not obtain EC Private Key")
			}
			bigSecret := new(big.Int).SetBytes(secret)

			tempSK := &ecdsa.PrivateKey{
				PublicKey: ecdsa.PublicKey{
					Curve: pubKey.Curve,
					X:     new(big.Int),
					Y:     new(big.Int),
				},
				D: new(big.Int),
			}

			var k = new(big.Int).SetBytes(reRandOpts.ExpansionValue())
			var one = new(big.Int).SetInt64(1)
			n := new(big.Int).Sub(pubKey.Params().N, one)
			k.Mod(k, n)
			k.Add(k, one)

			tempSK.D.Add(bigSecret, k)
			tempSK.D.Mod(tempSK.D, pubKey.Params().N)

			tempSK.PublicKey.X, tempSK.PublicKey.Y = pubKey.ScalarBaseMult(tempSK.D.Bytes())

			isOn := tempSK.Curve.IsOnCurve(tempSK.PublicKey.X, tempSK.PublicKey.Y)
			if !isOn {
				return nil, errors.New("Failed temporary public key IsOnCurve check.")
			}

			ecPt := elliptic.Marshal(tempSK.Curve, tempSK.X, tempSK.Y)
			oid, ok := oidFromNamedCurve(tempSK.Curve)
			if !ok {
				return nil, errors.New("Do not know OID for this Curve.")
			}

			ski, err := csp.importECKey(oid, tempSK.D.Bytes(), ecPt, opts.Ephemeral(), privateKeyFlag)
			if err != nil {
				return nil, fmt.Errorf("Failed getting importing EC Public Key [%s]", err)
			}
			reRandomizedKey := &ecdsaPrivateKey{ski, ecdsaPublicKey{ski, &tempSK.PublicKey}}

			return reRandomizedKey, nil

		default:
			return nil, fmt.Errorf("Unrecognized KeyDerivOpts provided [%s]", opts.Algorithm())

		}

	default:
		return csp.BCCSP.KeyDeriv(k, opts)

	}
}

func (csp *impl) KeyImport(raw interface{}, opts bccsp.KeyImportOpts) (k bccsp.Key, err error) {

	if raw == nil {
		return nil, errors.New("Invalid raw. Cannot be nil.")
	}

	if opts == nil {
		return nil, errors.New("Invalid Opts parameter. It must not be nil.")
	}

	switch opts.(type) {

	case *bccsp.ECDSAPKIXPublicKeyImportOpts:
		der, ok := raw.([]byte)
		if !ok {
			return nil, errors.New("[ECDSAPKIXPublicKeyImportOpts] Invalid raw material. Expected byte array.")
		}

		if len(der) == 0 {
			return nil, errors.New("[ECDSAPKIXPublicKeyImportOpts] Invalid raw. It must not be nil.")
		}

		lowLevelKey, err := utils.DERToPublicKey(der)
		if err != nil {
			return nil, fmt.Errorf("Failed converting PKIX to ECDSA public key [%s]", err)
		}

		ecdsaPK, ok := lowLevelKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, errors.New("Failed casting to ECDSA public key. Invalid raw material.")
		}

		ecPt := elliptic.Marshal(ecdsaPK.Curve, ecdsaPK.X, ecdsaPK.Y)
		oid, ok := oidFromNamedCurve(ecdsaPK.Curve)
		if !ok {
			return nil, errors.New("Do not know OID for this Curve.")
		}

		var ski []byte
		if csp.noPrivImport {
			hash := sha256.Sum256(ecPt)
			ski = hash[:]
		} else {
			if !csp.softVerify {
				logger.Debugf(LOGTABLE_BCCSP, "opencryptoki workaround warning: Importing public EC Key does not store out to pkcs11 store,\n"+
					"so verify with this key will fail, unless key is already present in store. Enable 'softwareverify'\n"+
					"in pkcs11 options, if suspect this issue.")
			}
			ski, err = csp.importECKey(oid, nil, ecPt, opts.Ephemeral(), publicKeyFlag)
			if err != nil {
				return nil, fmt.Errorf("Failed getting importing EC Public Key [%s]", err)
			}
		}

		k = &ecdsaPublicKey{ski, ecdsaPK}
		return k, nil

	case *bccsp.ECDSAPrivateKeyImportOpts:
		if csp.noPrivImport {
			return nil, errors.New("[ECDSADERPrivateKeyImportOpts] PKCS11 options 'sensitivekeys' is set to true. Cannot import.")
		}

		der, ok := raw.([]byte)
		if !ok {
			return nil, errors.New("[ECDSADERPrivateKeyImportOpts] Invalid raw material. Expected byte array.")
		}

		if len(der) == 0 {
			return nil, errors.New("[ECDSADERPrivateKeyImportOpts] Invalid raw. It must not be nil.")
		}

		lowLevelKey, err := utils.DERToPrivateKey(der)
		if err != nil {
			return nil, fmt.Errorf("Failed converting PKIX to ECDSA public key [%s]", err)
		}

		ecdsaSK, ok := lowLevelKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("Failed casting to ECDSA public key. Invalid raw material.")
		}

		ecPt := elliptic.Marshal(ecdsaSK.Curve, ecdsaSK.X, ecdsaSK.Y)
		oid, ok := oidFromNamedCurve(ecdsaSK.Curve)
		if !ok {
			return nil, errors.New("Do not know OID for this Curve.")
		}

		ski, err := csp.importECKey(oid, ecdsaSK.D.Bytes(), ecPt, opts.Ephemeral(), privateKeyFlag)
		if err != nil {
			return nil, fmt.Errorf("Failed getting importing EC Private Key [%s]", err)
		}

		k = &ecdsaPrivateKey{ski, ecdsaPublicKey{ski, &ecdsaSK.PublicKey}}
		return k, nil

	case *bccsp.ECDSAGoPublicKeyImportOpts:
		lowLevelKey, ok := raw.(*ecdsa.PublicKey)
		if !ok {
			return nil, errors.New("[ECDSAGoPublicKeyImportOpts] Invalid raw material. Expected *ecdsa.PublicKey.")
		}

		ecPt := elliptic.Marshal(lowLevelKey.Curve, lowLevelKey.X, lowLevelKey.Y)
		oid, ok := oidFromNamedCurve(lowLevelKey.Curve)
		if !ok {
			return nil, errors.New("Do not know OID for this Curve.")
		}

		var ski []byte
		if csp.noPrivImport {
			hash := sha256.Sum256(ecPt)
			ski = hash[:]
		} else {
			if !csp.softVerify {
				logger.Debugf(LOGTABLE_BCCSP, "opencryptoki workaround warning: Importing public EC Key does not store out to pkcs11 store,\n"+
					"so verify with this key will fail, unless key is already present in store. Enable 'softwareverify'\n"+
					"in pkcs11 options, if suspect this issue.")
			}
			ski, err = csp.importECKey(oid, nil, ecPt, opts.Ephemeral(), publicKeyFlag)
			if err != nil {
				return nil, fmt.Errorf("Failed getting importing EC Public Key [%s]", err)
			}
		}

		k = &ecdsaPublicKey{ski, lowLevelKey}
		return k, nil

	case *bccsp.X509PublicKeyImportOpts:
		x509Cert, ok := raw.(*x509.Certificate)
		if !ok {
			return nil, errors.New("[X509PublicKeyImportOpts] Invalid raw material. Expected *x509.Certificate.")
		}

		pk := x509Cert.PublicKey

		switch pk.(type) {
		case *ecdsa.PublicKey:
			return csp.KeyImport(pk, &bccsp.ECDSAGoPublicKeyImportOpts{Temporary: opts.Ephemeral()})
		case *rsa.PublicKey:
			return csp.KeyImport(pk, &bccsp.RSAGoPublicKeyImportOpts{Temporary: opts.Ephemeral()})
		default:
			return nil, errors.New("Certificate's public key type not recognized. Supported keys: [ECDSA, RSA]")
		}

	default:
		return csp.BCCSP.KeyImport(raw, opts)

	}
}

func (csp *impl) GetKey(ski []byte) (k bccsp.Key, err error) {
	pubKey, isPriv, err := csp.getECKey(ski)
	if err == nil {
		if isPriv {
			return &ecdsaPrivateKey{ski, ecdsaPublicKey{ski, pubKey}}, nil
		} else {
			return &ecdsaPublicKey{ski, pubKey}, nil
		}
	}
	return csp.BCCSP.GetKey(ski)
}

func (csp *impl) Sign(k bccsp.Key, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {

	if k == nil {
		return nil, errors.New("Invalid Key. It must not be nil.")
	}
	if len(digest) == 0 {
		return nil, errors.New("Invalid digest. Cannot be empty.")
	}

	switch k.(type) {
	case *ecdsaPrivateKey:
		return csp.signECDSA(*k.(*ecdsaPrivateKey), digest, opts)
	default:
		return csp.BCCSP.Sign(k, digest, opts)
	}
}

func (csp *impl) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {

	if k == nil {
		return false, errors.New("Invalid Key. It must not be nil.")
	}
	if len(signature) == 0 {
		return false, errors.New("Invalid signature. Cannot be empty.")
	}
	if len(digest) == 0 {
		return false, errors.New("Invalid digest. Cannot be empty.")
	}

	switch k.(type) {
	case *ecdsaPrivateKey:
		return csp.verifyECDSA(k.(*ecdsaPrivateKey).pub, signature, digest, opts)
	case *ecdsaPublicKey:
		return csp.verifyECDSA(*k.(*ecdsaPublicKey), signature, digest, opts)
	default:
		return csp.BCCSP.Verify(k, signature, digest, opts)
	}
}

func (csp *impl) Encrypt(k bccsp.Key, plaintext []byte, opts bccsp.EncrypterOpts) (ciphertext []byte, err error) {
	// TODO: Add PKCS11 support for encryption, when hnb starts requiring it
	return csp.BCCSP.Encrypt(k, plaintext, opts)
}

func (csp *impl) Decrypt(k bccsp.Key, ciphertext []byte, opts bccsp.DecrypterOpts) (plaintext []byte, err error) {
	return csp.BCCSP.Decrypt(k, ciphertext, opts)
}

func FindPKCS11Lib() (lib, pin, label string) {
	//FIXME: Till we workout the configuration piece, look for the libraries in the familiar places
	lib = os.Getenv("PKCS11_LIB")
	if lib == "" {
		pin = "98765432"
		label = "ForHGS"
		possibilities := []string{
			"/usr/lib/softhsm/libsofthsm2.so",                            //Debian
			"/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so",           //Ubuntu
			"/usr/lib/s390x-linux-gnu/softhsm/libsofthsm2.so",            //Ubuntu
			"/usr/lib/powerpc64le-linux-gnu/softhsm/libsofthsm2.so",      //Power
			"/usr/local/Cellar/softhsm/2.1.0/lib/softhsm/libsofthsm2.so", //MacOS
		}
		for _, path := range possibilities {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				lib = path
				break
			}
		}
	} else {
		pin = os.Getenv("PKCS11_PIN")
		label = os.Getenv("PKCS11_LABEL")
	}
	return lib, pin, label
}
