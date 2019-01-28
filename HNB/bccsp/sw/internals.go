package sw

import (
	"hash"

	"HNB/bccsp"
)

type KeyGenerator interface {
	KeyGen(opts bccsp.KeyGenOpts) (k bccsp.Key, err error)
}

type KeyDeriver interface {
	KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error)
}

type KeyImporter interface {
	KeyImport(raw interface{}, opts bccsp.KeyImportOpts) (k bccsp.Key, err error)
}

type Encryptor interface {
	Encrypt(k bccsp.Key, plaintext []byte, opts bccsp.EncrypterOpts) (ciphertext []byte, err error)
}

type Decryptor interface {
	Decrypt(k bccsp.Key, ciphertext []byte, opts bccsp.DecrypterOpts) (plaintext []byte, err error)
}

type Signer interface {
	Sign(k bccsp.Key, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error)
}

type Verifier interface {
	Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error)
}

type Hasher interface {
	Hash(msg []byte, opts bccsp.HashOpts) (hash []byte, err error)

	GetHash(opts bccsp.HashOpts) (h hash.Hash, err error)
}
