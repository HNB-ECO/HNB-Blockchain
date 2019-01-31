package vrf

import (
	"HNB/bccsp"
	"HNB/bccsp/factory"
	"HNB/bccsp/sw"
	"HNB/msp"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"HNB/bccsp/secp256k1"
)

var (
	ErrKeyNotSupported = errors.New("only support ECC and RSA key")
	ErrEvalVRF         = errors.New("failed to evaluate vrf")
	ErrInvalidVRF      = errors.New("invalid VRF proof")
	ErrInvalidHash     = errors.New("hash function does not match elliptic curve bitsize")
)

const (
	compress_even = 2
	compress_odd  = 3
	nocompress    = 4
)

//Vrf returns the verifiable random function evaluated m and a NIZK proof
func ComputeVRF(pri bccsp.Key, msg []byte, algType int) (vrf, nizk []byte, err error) {
	switch algType {
	case msp.ECDSAP256:
		sk := pri.(*sw.Ecdsa256K1PrivateKey)
		//sk := pri.(*sw.Ecdsa256K1PrivateKey)
		// Move key validation here
		isValid := ValidatePrivateKey(sk)
		if !isValid {
			return nil, nil, ErrKeyNotSupported
		}

		h := getHash(sk.PrivKey.Curve)
		byteLen := (sk.PrivKey.Params().BitSize + 7) >> 3
		_, proof := Evaluate(sk.PrivKey, h, msg)
		if proof == nil {
			return nil, nil, ErrEvalVRF
		}

		nizk = proof[0 : 2*byteLen]
		vrf = proof[2*byteLen : 2*byteLen+2*byteLen+1]
		err = nil
		return
	}

	return
}

func VerifyVRF(pubKey bccsp.Key, msg, vrf, nizk []byte, algType int) (bool, error) {
	switch algType {
	case msp.ECDSAP256:
		pk := pubKey.(*sw.Ecdsa256K1PublicKey)
		isValid := ValidatePublicKey(pk)
		if !isValid {
			return false, ErrKeyNotSupported
		}
		h := getHash(pk.PubKey.Curve)
		byteLen := (pk.PubKey.Params().BitSize + 7) >> 3
		if len(vrf) != byteLen*2+1 || len(nizk) != byteLen*2 {
			return false, nil
		}
		proof := append(nizk, vrf...)
		_, err := ProofToHash(pk.PubKey, h, msg, proof)
		if err != nil {
			fmt.Printf("xxx %v\n", err.Error())
			return false, err
		}
		return true, nil

	}

	return false, nil
}

type VRFBlkData struct {
	BlockNum uint64 `json:"block_num"`
	PrevVrf  []byte `json:"prev_vrf"`
}

type VRFBgData struct {
	BlockNum uint64 `json:"block_num"`
	BgNum    uint64 `json:"bg_num"`
	PrevVrf  []byte `json:"prev_vrf"`
}

func ComputeVRF4bg(sk bccsp.Key, VRFBgData *VRFBgData, algType int) (VRFValue, VRFProof []byte, err error) {
	data, err := json.Marshal(VRFBgData)
	if err != nil {
		return nil, nil, fmt.Errorf("computeVrf failed to marshal VRFBgData: %s", err)
	}

	return ComputeVRF(sk, data, algType)
}

func VerifyVRF4bg(pk bccsp.Key, VRFBgData *VRFBgData, VRFValue, VRFProof []byte, algType int) (bool, error) {
	data, err := json.Marshal(VRFBgData)
	if err != nil {
		return false, fmt.Errorf("computeVrf failed to marshal VRFBgData: %s", err)
	}

	return VerifyVRF(pk, data, VRFValue, VRFProof, algType)
}

// 给新出的块计算VRFValue和VRFProof
func ComputeVRF4Blk(sk bccsp.Key, VRFBlkData *VRFBlkData, algType int) (VRFValue, VRFProof []byte, err error) {
	data, err := json.Marshal(VRFBlkData)
	if err != nil {
		return nil, nil, fmt.Errorf("computeVrf failed to marshal VRFBlkData: %s", err)
	}

	return ComputeVRF(sk, data, algType)
}

func VerifyVRF4Blk(pk bccsp.Key, VRFBlkData *VRFBlkData, VRFValue, VRFProof []byte, algType int) (bool, error) {
	data, err := json.Marshal(VRFBlkData)
	if err != nil {
		return false, fmt.Errorf("computeVrf failed to marshal VRFBlkData: %s", err)
	}

	return VerifyVRF(pk, data, VRFValue, VRFProof, algType)
}

func ValidatePrivateKey(pri bccsp.Key) bool {
	switch t := pri.(type) {
	case *sw.Ecdsa256K1PrivateKey:
		h := getHash(t.PrivKey.Curve)
		if h == nil {
			return false
		}
		return true

		return true
	default:
		return false
	}
}

/*
 * ValidatePublicKey checks two conditions:
 *  - the public key must be of type ec.PublicKey
 *	- the public key must use curve secp256r1
 */
func ValidatePublicKey(pk bccsp.Key) bool {
	switch t := pk.(type) {
	case *sw.Ecdsa256K1PublicKey:
		h := getHash(t.PubKey.Curve)
		if h == nil {
			return false
		}
		return true
	default:
		return false
	}
}

func getHash(curve elliptic.Curve) hash.Hash {
	bitSize := curve.Params().BitSize
	switch bitSize {

	case 256:
		hash, err := factory.GetDefault().GetHash(&bccsp.SHA256Opts{})
		if err != nil {
			return nil //todo
		}
		return hash
	default:
		return nil
	}

	return nil
}

// Evaluate returns the verifiable unpredictable(random) function evaluated at m
func Evaluate(pri *ecdsa.PrivateKey, h hash.Hash, m []byte) (index [32]byte, proof []byte) {
	curve := pri.Curve.(*secp256k1.BitCurve)
	nilIndex := [32]byte{}

	byteLen := (curve.BitSize + 7) >> 3
	if byteLen != h.Size() {
		return nilIndex, nil
	}
	// Prover chooses r <-- [1,N-1]
	r, _, _, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nilIndex, nil
	}
	ri := new(big.Int).SetBytes(r)

	// H = hashToCurve(pk || m)
	var buf bytes.Buffer
	buf.Write(curve.Marshal(pri.PublicKey.X, pri.PublicKey.Y))
	buf.Write(m)
	Hx, Hy := hashToCurve(curve, h, buf.Bytes())
	// VRF_pri(m) = [pri]H
	sHx, sHy := curve.ScalarMult(Hx, Hy, pri.D.Bytes())
	if curve.IsOnCurve(sHx, sHy) == false {
		panic("sssss")
	}
	vrf := curve.Marshal(sHx, sHy) // 2*byteLen+1 bytes.

	// G is the base point
	// s = hashToInt(G, H, [pri]G, VRF, [r]G, [r]H)
	rGx, rGy := curve.ScalarBaseMult(r)
	rHx, rHy := curve.ScalarMult(Hx, Hy, r)
	var b bytes.Buffer
	b.Write(curve.Marshal(curve.Gx, curve.Gy))
	b.Write(curve.Marshal(Hx, Hy))
	b.Write(curve.Marshal(pri.PublicKey.X, pri.PublicKey.Y))
	b.Write(vrf)
	b.Write(curve.Marshal(rGx, rGy))
	b.Write(curve.Marshal(rHx, rHy))
	s := hashToInt(curve, h, b.Bytes())

	// t = r−s*pri mod N
	t := new(big.Int).Sub(ri, new(big.Int).Mul(s, pri.D))
	t.Mod(t, curve.N)

	// Index = SHA256(vrf)
	index = sha256.Sum256(vrf)

	// Write s, t, and vrf to a proof blob. Also write leading zeros before s and t
	// if needed.
	buf.Reset()
	buf.Write(make([]byte, byteLen-len(s.Bytes())))
	buf.Write(s.Bytes())
	buf.Write(make([]byte, byteLen-len(t.Bytes())))
	buf.Write(t.Bytes())
	buf.Write(vrf) //byteLen*2 + byteLen*2 + 1

	return index, buf.Bytes()
}

// hashToCurve hashes to a point on elliptic curve
func hashToCurve(curve elliptic.Curve, h hash.Hash, m []byte) (x, y *big.Int) {
	//var i uint32
	params := curve.(*secp256k1.BitCurve)
	//byteLen := (params.BitSize + 7) >> 3

	h.Reset()
	h.Write(m)
	r := h.Sum(nil)

	return params.ScalarBaseMult(r)

	//for x == nil && i < 100 {
	//	// TODO: Use a NIST specified DRBG.
	//	h.Reset()
	//	binary.Write(h, binary.BigEndian, i)
	//	h.Write(m)
	//	r := []byte{2} // Set point encoding to "compressed", y=0.
	//	r = h.Sum(r)
	//	p, err := DecodePublicKey(r[:byteLen+1], curve)
	//	if err != nil {
	//		x, y = nil, nil
	//	} else {
	//		x, y = p.X, p.Y
	//	}
	//	i++
	//}
	//return
}

func DecodePublicKey(data []byte, curve elliptic.Curve) (*ecdsa.PublicKey, error) {
	if curve == nil {
		return nil, errors.New("unknown curve")
	}

	length := (curve.Params().BitSize + 7) >> 3
	if len(data) < length+1 {
		return nil, errors.New("invalid data length")
	}

	var x, y *big.Int
	x = new(big.Int).SetBytes(data[1 : length+1])
	if data[0] == nocompress {
		if len(data) < length*2+1 {
			return nil, errors.New("invalid data length")
		}
		y = new(big.Int).SetBytes(data[length+1 : length*2+1])
		//TODO verify whether (x,y) is on the curve
		if !curve.IsOnCurve(x, y) {
			return nil, errors.New("Point is not on the curve")
		}
	} else if data[0] == compress_even || data[0] == compress_odd {
		return deCompress(int(data[0]&1), data[1:length+1], curve)
	} else {
		return nil, errors.New("unknown encoding mode")
	}

	return &ecdsa.PublicKey{
		X:     x,
		Y:     y,
		Curve: curve,
	}, nil
}

var one = big.NewInt(1)

// hashToInt hashes to an integer [1,N-1]
func hashToInt(curve elliptic.Curve, h hash.Hash, m []byte) *big.Int {
	// NIST SP 800-90A § A.5.1: Simple discard method.
	params := curve.(*secp256k1.BitCurve)
	byteLen := (params.BitSize + 7) >> 3
	for i := uint32(0); ; i++ {
		// TODO: Use a NIST specified DRBG.
		h.Reset()
		binary.Write(h, binary.BigEndian, i)
		h.Write(m)
		b := h.Sum(nil)
		k := new(big.Int).SetBytes(b[:byteLen])
		if k.Cmp(new(big.Int).Sub(params.N, one)) == -1 {
			return k.Add(k, one)
		}
	}
}

// ProofToHash asserts that proof is correct for m and outputs index.
func ProofToHash(pk *ecdsa.PublicKey, h hash.Hash, m, proof []byte) (index [32]byte, err error) {
	nilIndex := [32]byte{}
	curve := pk.Curve.(*secp256k1.BitCurve)
	byteLen := (curve.BitSize + 7) >> 3
	if byteLen != h.Size() {
		return nilIndex, ErrInvalidHash
	}

	// verifier checks that s == hashToInt(m, [t]G + [s]([pri]G), [t]hashToCurve(pk, m) + [s]VRF_pri(m))
	if got, want := len(proof), (2*byteLen)+(2*byteLen+1); got != want {
		return nilIndex, ErrInvalidVRF
	}

	// Parse proof into s, t, and vrf.
	s := proof[0:byteLen]
	t := proof[byteLen : 2*byteLen]
	vrf := proof[2*byteLen : 2*byteLen+2*byteLen+1]

	uHx, uHy := curve.Unmarshal(vrf)
	if uHx == nil {
		return nilIndex, ErrInvalidVRF
	}

	// [t]G + [s]([pri]G) = [t+pri*s]G
	tGx, tGy := curve.ScalarBaseMult(t)
	ksGx, ksGy := curve.ScalarMult(pk.X, pk.Y, s)
	tksGx, tksGy := curve.Add(tGx, tGy, ksGx, ksGy)

	// H = hashToCurve(pk || m)
	// [t]H + [s]VRF = [t+pri*s]H
	buf := new(bytes.Buffer)
	buf.Write(curve.Marshal(pk.X, pk.Y))
	buf.Write(m)
	Hx, Hy := hashToCurve(pk.Curve, h, buf.Bytes())

	tHx, tHy := curve.ScalarMult(Hx, Hy, t)
	sHx, sHy := curve.ScalarMult(uHx, uHy, s)
	tksHx, tksHy := curve.Add(tHx, tHy, sHx, sHy)

	//   hashToInt(G, H, [pri]G, VRF, [t]G + [s]([pri]G), [t]H + [s]VRF)
	// = hashToInt(G, H, [pri]G, VRF, [t+pri*s]G, [t+pri*s]H)
	// = hashToInt(G, H, [pri]G, VRF, [r]G, [r]H)
	var b bytes.Buffer
	b.Write(curve.Marshal(curve.Gx, curve.Gy))
	b.Write(curve.Marshal(Hx, Hy))
	b.Write(curve.Marshal(pk.X, pk.Y))
	b.Write(vrf)
	b.Write(curve.Marshal(tksGx, tksGy))
	b.Write(curve.Marshal(tksHx, tksHy))

	h2 := hashToInt(curve, h, b.Bytes())

	// Left pad h2 with zeros if needed. This will ensure that h2 is padded
	// the same way s is.
	buf.Reset()
	buf.Write(make([]byte, byteLen-len(h2.Bytes())))
	buf.Write(h2.Bytes())

	if !hmac.Equal(s, buf.Bytes()) {
		return nilIndex, ErrInvalidVRF
	}
	return sha256.Sum256(vrf), nil
}
