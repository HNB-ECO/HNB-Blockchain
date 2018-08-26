package txpool

import(
	"testing"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
)

func TestNewTXPoolServer(t *testing.T) {

}

func TestRecvTx(t *testing.T) {
	logging.InitLogModule()
	p2pNetwork.NewServer().Start()
	tps := NewTXPoolServer()
	tps.InitTXPoolServer()
	tx := &common.Transaction{
		Txid: "lala",
		Type: HGS,
	}
	data, _ := json.Marshal(tx)
	tps.RecvTx(data)
	tpw, _ := tps.GetServer(HGS)
	txs, _ := tpw.GetTxsFromTXPool(3)
	fmt.Println(txs)
}
