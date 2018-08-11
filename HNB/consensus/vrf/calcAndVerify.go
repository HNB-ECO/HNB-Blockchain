package vrf



import (
	"encoding/json"
	"fmt"
	"crypto"
	"Test/vrf/common"
)

func computeVrf(sk common.PrivateKey, blkNum uint32, preVrf []byte) ([]byte, []byte, error)  {
	vd := &common.VrfData{
		BlockNum:blkNum,
		PrevVrf:preVrf,
	}
	data, err := json.Marshal(vd)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal err %s", err.Error())
	}
	return common.Vrf(sk, data)
}

func verify(pk crypto.PublicKey, blkNum uint32, preVrf, newVrfValue, newVrfProof []byte) error {
	vd := &common.VrfData{
		BlockNum:blkNum,
		PrevVrf:preVrf,
	}
	data, err := json.Marshal(vd)
	if err != nil {
		return fmt.Errorf("marshal err %s", err.Error())
	}
	result, err := common.VerifyVrf(pk, data,newVrfProof, newVrfValue)
	if err != nil {
		return err
	}
	if !result {
		return fmt.Errorf("varify failed")
	}
	return nil
}

