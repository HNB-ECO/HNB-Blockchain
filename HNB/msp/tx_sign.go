package msp

import (
	"HNB/bccsp"
	"HNB/bccsp/secp256k1"
	"HNB/bccsp/sha3"
	"HNB/bccsp/sw"
	"HNB/common"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	//	"HNB/config"
)

var (
	ErrInvalidChainId = errors.New("invalid chain id for signer")
	ErrInvalidSig     = errors.New("invalid transaction v, r, s values")
)

var (
	Big1   = big.NewInt(1)
	Big2   = big.NewInt(2)
	Big3   = big.NewInt(3)
	Big0   = big.NewInt(0)
	Big32  = big.NewInt(32)
	Big256 = big.NewInt(256)
	Big257 = big.NewInt(257)
)

var (
	secp256k1N, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))
)

var hnbSigner *HNBSigner

func GetSigner() Signer {
	if hnbSigner != nil {
		return hnbSigner
	}

	chainID := new(big.Int)
	chainID.SetBytes([]byte(common.HNB))
	hnbSigner = NewHNBSigner(chainID)

	return hnbSigner
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *common.Transaction, s Signer) (*common.Transaction, error) {
	//txMarshal, _ := json.Marshal(tx)
	hash := s.Hash(tx)
	sig, err := SignWithHash(hash.GetBytes())
	if err != nil {
		return nil, err
	}

	err = WithSignature(tx, s, sig)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func WithSignature(tx *common.Transaction, signer Signer, sig []byte) error {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return err
	}

	tx.R = r
	tx.S = s
	tx.V = v

	return nil
}

func Sender(signer Signer, tx *common.Transaction) (common.Address, error) {
	//TODO 添加缓存 每次计算签名，tps过低

	addr, err := signer.Sender(tx)
	if err != nil {
		return common.Address{}, err
	}

	return addr, nil
}

type Signer interface {
	Sender(tx *common.Transaction) (common.Address, error)
	SignatureValues(tx *common.Transaction, sig []byte) (r, s, v *big.Int, err error)
	Hash(tx *common.Transaction) common.Hash
	Equal(Signer) bool
}

// EIP155Transaction implements Signer using the EIP155 rules.
type HNBSigner struct {
	chainId, chainIdMul *big.Int
}

func NewHNBSigner(chainId *big.Int) *HNBSigner {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return &HNBSigner{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
}

func (s HNBSigner) Equal(s2 Signer) bool {
	eip155, ok := s2.(HNBSigner)
	return ok && eip155.chainId.Cmp(s.chainId) == 0
}

var big8 = big.NewInt(8)

func Protected(tx *common.Transaction) bool {
	return isProtectedV(tx.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 is considered protected
	return true
}

func (s HNBSigner) Sender(tx *common.Transaction) (common.Address, error) {
	//if strings.Compare(tx.ContractName, string(s.chainId.Bytes())) != 0 {
	//	return common.Address{}, ErrInvalidChainId
	//}
	if !Protected(tx) {
		return HomesteadSigner{}.Sender(tx)
	}

	V := new(big.Int).Sub(tx.V, s.chainIdMul)
	V.Sub(V, big8)
	return recoverPlain(s.Hash(tx), tx.R, tx.S, V, true)
}

func (s HNBSigner) SignatureValues(tx *common.Transaction, sig []byte) (R, S, V *big.Int, err error) {
	R, S, V, err = HomesteadSigner{}.SignatureValues(tx, sig)
	if err != nil {
		return nil, nil, nil, err
	}
	if s.chainId.Sign() != 0 {
		V = big.NewInt(int64(sig[64] + 35))
		V.Add(V, s.chainIdMul)
	}
	return R, S, V, nil
}

func signHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	m, _ := json.Marshal(x)
	hw.Write(m)
	hw.Sum(h[:0])
	return h
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s HNBSigner) Hash(tx *common.Transaction) (h common.Hash) {
	//	if config.Config.RunMode == "dev"{
	return signHash([]interface{}{
		tx.ContractName,
		tx.From,
		tx.Payload,
		tx.NonceValue,
	})
	//}
	//return signHash([]interface{}{
	//	tx.ContractName,
	//	tx.From,
	//	tx.Payload,
	//	0,
	//})
}

// HomesteadTransaction implements TransactionInterface using the
// homestead rules.
type HomesteadSigner struct{ FrontierSigner }

func (s HomesteadSigner) Equal(s2 Signer) bool {
	_, ok := s2.(HomesteadSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (hs HomesteadSigner) SignatureValues(tx *common.Transaction, sig []byte) (r, s, v *big.Int, err error) {
	return hs.FrontierSigner.SignatureValues(tx, sig)
}

func (hs HomesteadSigner) Sender(tx *common.Transaction) (common.Address, error) {
	return recoverPlain(hs.Hash(tx), tx.R, tx.S, tx.V, true)
}

type FrontierSigner struct{}

func (s FrontierSigner) Equal(s2 Signer) bool {
	_, ok := s2.(FrontierSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (fs FrontierSigner) SignatureValues(tx *common.Transaction, sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (fs FrontierSigner) Hash(tx *common.Transaction) common.Hash {
	//if config.Config.RunMode == "dev"{
	return signHash([]interface{}{
		tx.ContractName,
		tx.From,
		tx.Payload,
		tx.NonceValue,
	})
	//}
	//
	//return signHash([]interface{}{
	//	tx.ContractName,
	//	tx.From,
	//	tx.Payload,
	//	0,
	//})

}

func (fs FrontierSigner) Sender(tx *common.Transaction) (common.Address, error) {
	return recoverPlain(fs.Hash(tx), tx.R, tx.S, tx.V, false)
}

func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	if r.Cmp(Big1) < 0 || s.Cmp(Big1) < 0 {
		return false
	}
	// reject upper range of s values (ECDSA malleability)
	// see discussion in secp256k1/libsecp256k1/include/secp256k1.h
	if homestead && s.Cmp(secp256k1halfN) > 0 {
		return false
	}
	// Frontier: allow s to be in full N range
	return r.Cmp(secp256k1N) < 0 && s.Cmp(secp256k1N) < 0 && (v == 0 || v == 1)
}

func Ecrecover(hash, sig []byte) ([]byte, error) {
	return secp256k1.RecoverPubkey(hash, sig)
}

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	pub, err := Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], Keccak256(pub[1:])[12:])
	return addr, nil
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(secp256k1.S256(), pub.X, pub.Y)
}

func BytesToAddress(b []byte) common.Address {
	var a common.Address
	a.SetBytes(b)
	return a
}

func AccountPubkeyToAddress() common.Address {
	key := keyPair.PubKey.(*sw.Ecdsa256K1PublicKey).PubKey
	pubBytes := FromECDSAPub(key)
	return BytesToAddress(Keccak256(pubBytes[1:])[12:])
}

func AccountPubkeyToAddress1(pubkey bccsp.Key) common.Address {
	key := pubkey.(*sw.Ecdsa256K1PublicKey).PubKey

	pubBytes := FromECDSAPub(key)

	return BytesToAddress(Keccak256(pubBytes[1:])[12:])
}
