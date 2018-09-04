package ledger

import(
	ssComm "HNB/ledger/stateStore/common"
	"encoding/binary"
)

var ledgerHeight uint64 = 0

const(
	HEIGHTKEY = "height"
)
func SetState(key []byte, state *ssComm.StateSet){
	//保存每次操作的原始数据和新数据，为了处理回滚
}

func GetStateSet(key []byte) *ssComm.StateSet{
	//回滚时使用
	return nil
}


func SetBlockHeight(height uint64) error{
	//b := make([]byte, 8)
	//binary.BigEndian.PutUint64(b, i)
	//
	//fmt.Println(b[:])
	//
	//i = uint64(binary.BigEndian.Uint64(b))
	defer func(){
		ledgerHeight = height
		LedgerLog.Infof(LOGTABLE_LEDGER,"set block height %v", height)
	}()
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, height)
	return lh.dbHandler.Put([]byte(HEIGHTKEY), b)
}

func GetBlockHeight() (uint64, error){
	h,err := lh.dbHandler.Get([]byte(HEIGHTKEY))
	if err != nil{
		return 0,  err
	}

	if h == nil{
		return 0, nil
	}
	height := uint64(binary.BigEndian.Uint64(h))
	ledgerHeight = height
	return height, nil
}