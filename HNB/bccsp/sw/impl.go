package sw

import (
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"reflect"

	"HNB/bccsp"
	"HNB/bccsp/errors"
	"HNB/logging"
	"golang.org/x/crypto/sha3"
)

var (
	logger logging.LogModule
)

func init() {
	logger = logging.GetLogIns()
}

const LOGTABLE_BCCSP string = "bccsp"

func NewDefaultSecurityLevel(keyStorePath string) (bccsp.BCCSP, error) {
	ks := &fileBasedKeyStore{}
	if err := ks.Init(nil, keyStorePath, false); err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed initializing key store at [%v]", keyStorePath).WrapError(err)
	}

	return New(256, "SHA2", ks)
}

func NewDefaultSecurityLevelWithKeystore(keyStore bccsp.KeyStore) (bccsp.BCCSP, error) {
	return New(256, "SHA2", keyStore)
}

func New(securityLevel int, hashFamily string, keyStore bccsp.KeyStore) (bccsp.BCCSP, error) {
	conf := &config{}
	err := conf.setSecurityLevel(securityLevel, hashFamily)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed initializing configuration at [%v,%v]", securityLevel, hashFamily).WrapError(err)
	}

	if keyStore == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid bccsp.KeyStore instance. It must be different from nil.")
	}

	encryptors := make(map[reflect.Type]Encryptor)
	encryptors[reflect.TypeOf(&aesPrivateKey{})] = &aescbcpkcs7Encryptor{}

	decryptors := make(map[reflect.Type]Decryptor)
	decryptors[reflect.TypeOf(&aesPrivateKey{})] = &aescbcpkcs7Decryptor{}

	signers := make(map[reflect.Type]Signer)
	signers[reflect.TypeOf(&EcdsaPrivateKey{})] = &ecdsaSigner{}
	signers[reflect.TypeOf(&rsaPrivateKey{})] = &rsaSigner{}
	signers[reflect.TypeOf(&Ecdsa256K1PrivateKey{})] = &ecdsa256K1Signer{}

	verifiers := make(map[reflect.Type]Verifier)

	verifiers[reflect.TypeOf(&Ecdsa256K1PublicKey{})] = &ecdsaPublicKey256K1Verifier{}
	verifiers[reflect.TypeOf(&Ecdsa256K1PrivateKey{})] = &ecdsaPrivKey256K1Verifier{}

	verifiers[reflect.TypeOf(&EcdsaPrivateKey{})] = &ecdsaPrivateKeyVerifier{}
	verifiers[reflect.TypeOf(&EcdsaPublicKey{})] = &ecdsaPublicKeyKeyVerifier{}
	verifiers[reflect.TypeOf(&rsaPrivateKey{})] = &rsaPrivateKeyVerifier{}
	verifiers[reflect.TypeOf(&rsaPublicKey{})] = &rsaPublicKeyKeyVerifier{}

	hashers := make(map[reflect.Type]Hasher)
	hashers[reflect.TypeOf(&bccsp.SHAOpts{})] = &hasher{hash: conf.hashFunction}
	hashers[reflect.TypeOf(&bccsp.SHA256Opts{})] = &hasher{hash: sha256.New}
	hashers[reflect.TypeOf(&bccsp.SHA384Opts{})] = &hasher{hash: sha512.New384}
	hashers[reflect.TypeOf(&bccsp.SHA3_256Opts{})] = &hasher{hash: sha3.New256}
	hashers[reflect.TypeOf(&bccsp.SHA3_384Opts{})] = &hasher{hash: sha3.New384}

	impl := &impl{
		conf:       conf,
		ks:         keyStore,
		encryptors: encryptors,
		decryptors: decryptors,
		signers:    signers,
		verifiers:  verifiers,
		hashers:    hashers}

	keyGenerators := make(map[reflect.Type]KeyGenerator)

	keyGenerators[reflect.TypeOf(&bccsp.ECDSAP256K1KeyGenOpts{})] = &ecdsa256K1KeyGenerator{}
	keyGenerators[reflect.TypeOf(&bccsp.ECDSAKeyGenOpts{})] = &ecdsaKeyGenerator{curve: conf.ellipticCurve}
	keyGenerators[reflect.TypeOf(&bccsp.ECDSAP256KeyGenOpts{})] = &ecdsaKeyGenerator{curve: elliptic.P256()}
	keyGenerators[reflect.TypeOf(&bccsp.ECDSAP384KeyGenOpts{})] = &ecdsaKeyGenerator{curve: elliptic.P384()}
	keyGenerators[reflect.TypeOf(&bccsp.AESKeyGenOpts{})] = &aesKeyGenerator{length: conf.aesBitLength}
	keyGenerators[reflect.TypeOf(&bccsp.AES256KeyGenOpts{})] = &aesKeyGenerator{length: 32}
	keyGenerators[reflect.TypeOf(&bccsp.AES192KeyGenOpts{})] = &aesKeyGenerator{length: 24}
	keyGenerators[reflect.TypeOf(&bccsp.AES128KeyGenOpts{})] = &aesKeyGenerator{length: 16}
	keyGenerators[reflect.TypeOf(&bccsp.RSAKeyGenOpts{})] = &rsaKeyGenerator{length: conf.rsaBitLength}
	keyGenerators[reflect.TypeOf(&bccsp.RSA1024KeyGenOpts{})] = &rsaKeyGenerator{length: 1024}
	keyGenerators[reflect.TypeOf(&bccsp.RSA2048KeyGenOpts{})] = &rsaKeyGenerator{length: 2048}
	keyGenerators[reflect.TypeOf(&bccsp.RSA3072KeyGenOpts{})] = &rsaKeyGenerator{length: 3072}
	keyGenerators[reflect.TypeOf(&bccsp.RSA4096KeyGenOpts{})] = &rsaKeyGenerator{length: 4096}
	impl.keyGenerators = keyGenerators

	keyDerivers := make(map[reflect.Type]KeyDeriver)
	keyDerivers[reflect.TypeOf(&EcdsaPrivateKey{})] = &ecdsaPrivateKeyKeyDeriver{}
	keyDerivers[reflect.TypeOf(&EcdsaPublicKey{})] = &ecdsaPublicKeyKeyDeriver{}
	keyDerivers[reflect.TypeOf(&aesPrivateKey{})] = &aesPrivateKeyKeyDeriver{bccsp: impl}
	impl.keyDerivers = keyDerivers

	keyImporters := make(map[reflect.Type]KeyImporter)

	keyImporters[reflect.TypeOf(&bccsp.AES256ImportKeyOpts{})] = &aes256ImportKeyOptsKeyImporter{}
	keyImporters[reflect.TypeOf(&bccsp.HMACImportKeyOpts{})] = &hmacImportKeyOptsKeyImporter{}
	keyImporters[reflect.TypeOf(&bccsp.ECDSAPKIXPublicKeyImportOpts{})] = &ecdsaPKIXPublicKeyImportOptsKeyImporter{}
	keyImporters[reflect.TypeOf(&bccsp.ECDSAPrivateKeyImportOpts{})] = &ecdsaPrivateKeyImportOptsKeyImporter{}
	keyImporters[reflect.TypeOf(&bccsp.ECDSAGoPublicKeyImportOpts{})] = &ecdsaGoPublicKeyImportOptsKeyImporter{}
	keyImporters[reflect.TypeOf(&bccsp.RSAGoPublicKeyImportOpts{})] = &rsaGoPublicKeyImportOptsKeyImporter{}
	keyImporters[reflect.TypeOf(&bccsp.X509PublicKeyImportOpts{})] = &x509PublicKeyImportOptsKeyImporter{bccsp: impl}
	keyImporters[reflect.TypeOf(&bccsp.ECDSAPrivateKey256K1ImportOpts{})] = &ecdsaPrivateKey256K1ImportOptsKeyImporter{}

	impl.keyImporters = keyImporters

	return impl, nil
}

type impl struct {
	conf *config
	ks   bccsp.KeyStore

	keyGenerators map[reflect.Type]KeyGenerator
	keyDerivers   map[reflect.Type]KeyDeriver
	keyImporters  map[reflect.Type]KeyImporter
	encryptors    map[reflect.Type]Encryptor
	decryptors    map[reflect.Type]Decryptor
	signers       map[reflect.Type]Signer
	verifiers     map[reflect.Type]Verifier
	hashers       map[reflect.Type]Hasher
}

func (csp *impl) KeyGen(opts bccsp.KeyGenOpts) (k bccsp.Key, err error) {
	if opts == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid Opts parameter. It must not be nil.")
	}

	keyGenerator, found := csp.keyGenerators[reflect.TypeOf(opts)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'KeyGenOpts' provided [%v]", opts)
	}

	k, err = keyGenerator.KeyGen(opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed generating key with opts [%v]", opts).WrapError(err)
	}

	if !opts.Ephemeral() {
		err = csp.ks.StoreKey(k)
		if err != nil {
			return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed storing key [%s]. [%s]", opts.Algorithm(), err)
		}
	}

	return k, nil
}

func (csp *impl) KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error) {

	if k == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid Key. It must not be nil.")
	}
	if opts == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid opts. It must not be nil.")
	}

	keyDeriver, found := csp.keyDerivers[reflect.TypeOf(k)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'Key' provided [%v]", k)
	}

	k, err = keyDeriver.KeyDeriv(k, opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed deriving key with opts [%v]", opts).WrapError(err)
	}

	if !opts.Ephemeral() {
		err = csp.ks.StoreKey(k)
		if err != nil {
			return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed storing key [%s]. [%s]", opts.Algorithm(), err)
		}
	}

	return k, nil
}

func (csp *impl) KeyImport(raw interface{}, opts bccsp.KeyImportOpts) (k bccsp.Key, err error) {

	if raw == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid raw. It must not be nil.")
	}
	if opts == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid opts. It must not be nil.")
	}

	keyImporter, found := csp.keyImporters[reflect.TypeOf(opts)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'KeyImportOpts' provided [%v]", opts)
	}

	k, err = keyImporter.KeyImport(raw, opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed importing key with opts [%v]", opts).WrapError(err)
	}

	if !opts.Ephemeral() {
		err = csp.ks.StoreKey(k)
		if err != nil {
			return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed storing imported key with opts [%v]", opts).WrapError(err)
		}
	}

	return
}

func (csp *impl) GetKey(ski []byte) (k bccsp.Key, err error) {
	k, err = csp.ks.GetKey(ski)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed getting key for SKI [%v]", ski).WrapError(err)
	}

	return
}

func (csp *impl) Hash(msg []byte, opts bccsp.HashOpts) (digest []byte, err error) {

	if opts == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid opts. It must not be nil.")
	}

	hasher, found := csp.hashers[reflect.TypeOf(opts)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'HashOpt' provided [%v]", opts)
	}

	digest, err = hasher.Hash(msg, opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed hashing with opts [%v]", opts).WrapError(err)
	}

	return
}

func (csp *impl) GetHash(opts bccsp.HashOpts) (h hash.Hash, err error) {
	if opts == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid opts. It must not be nil.")
	}

	hasher, found := csp.hashers[reflect.TypeOf(opts)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'HashOpt' provided [%v]", opts)
	}

	h, err = hasher.GetHash(opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed getting hash function with opts [%v]", opts).WrapError(err)
	}

	return
}

func (csp *impl) Sign(k bccsp.Key, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {
	if k == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid Key. It must not be nil.")
	}
	if len(digest) == 0 {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid digest. Cannot be empty.")
	}

	signer, found := csp.signers[reflect.TypeOf(k)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'SignKey' provided [%v]", k)
	}

	signature, err = signer.Sign(k, digest, opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed signing with opts [%v]", opts).WrapError(err)
	}

	return
}

func (csp *impl) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {
	if k == nil {
		return false, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid Key. It must not be nil.")
	}
	if len(signature) == 0 {
		return false, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid signature. Cannot be empty.")
	}
	if len(digest) == 0 {
		return false, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid digest. Cannot be empty.")
	}

	verifier, found := csp.verifiers[reflect.TypeOf(k)]
	if !found {
		return false, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'VerifyKey' provided [%v]", k)
	}

	valid, err = verifier.Verify(k, signature, digest, opts)
	if err != nil {
		return false, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed verifing with opts [%v]", opts).WrapError(err)
	}

	return
}

func (csp *impl) Encrypt(k bccsp.Key, plaintext []byte, opts bccsp.EncrypterOpts) (ciphertext []byte, err error) {
	if k == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid Key. It must not be nil.")
	}

	encryptor, found := csp.encryptors[reflect.TypeOf(k)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'EncryptKey' provided [%v]", k)
	}

	return encryptor.Encrypt(k, plaintext, opts)
}

func (csp *impl) Decrypt(k bccsp.Key, ciphertext []byte, opts bccsp.DecrypterOpts) (plaintext []byte, err error) {
	if k == nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.BadRequest, "Invalid Key. It must not be nil.")
	}

	decryptor, found := csp.decryptors[reflect.TypeOf(k)]
	if !found {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.NotFound, "Unsupported 'DecryptKey' provided [%v]", k)
	}

	plaintext, err = decryptor.Decrypt(k, ciphertext, opts)
	if err != nil {
		return nil, errors.ErrorWithCallstack(errors.BCCSP, errors.Internal, "Failed decrypting with opts [%v]", opts).WrapError(err)
	}

	return
}
