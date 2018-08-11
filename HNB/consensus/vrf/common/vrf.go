package common

import (
	"crypto"
	"crypto/ecdsa"
	"github.com/google/keytransparency/core/crypto/vrf/p256"
	"fmt"
)

type VrfData struct {
	BlockNum uint32 `json:"block_num"`
	PrevVrf  []byte `json:"prev_vrf"`
}

type PrivateKey interface {
	crypto.PrivateKey
	Public() crypto.PublicKey
}

func Vrf(pri PrivateKey, msg []byte) ([]byte, []byte, error) {
	t := pri.(*ecdsa.PrivateKey)
	sk := new(p256.PrivateKey)
	sk.PrivateKey = t
	_, proof := sk.Evaluate(msg)
	if proof == nil || len(proof) != 64+65 {
		return nil, nil, fmt.Errorf("proof err")
	}
	return proof[64:64+65], proof[:64], nil
}

func VerifyVrf(pk crypto.PublicKey, msg, vrfProof, vrfValue []byte) (bool, error)  {
	t := pk.(*ecdsa.PublicKey)
	pubKey := new(p256.PublicKey)
	pubKey.PublicKey = t
	if len(vrfProof) != 64 || len(vrfValue) != 65 {
		return false, fmt.Errorf("vrf len err len(vrfProof)=%d, len(vrfValue)=%d",len(vrfProof), len(vrfValue))
	}
	proof := append(vrfProof, vrfValue...)
	_, err := pubKey.ProofToHash(msg, proof)
	if err != nil {
		return false, err
	}
	return true, nil
}