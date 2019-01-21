package cli

import (
	"HNB/bccsp"
	"HNB/bccsp/secp256k1"
	"HNB/bccsp/sw"
	"HNB/common"
	"HNB/contract/hgs"
	"HNB/contract/hnb"
	"HNB/msp"
	"HNB/txpool"
	"HNB/util"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"math/big"
	"net/http"
	"HNB/consensus/dbft"
	"HNB/access/rest"
	"HNB/rlp"
)

var (
	CliTxSendMsg = cli.StringFlag{
		Name:  "sendmsg",
		Value: "Hello World",
		Usage: "Send the cli interface of the transaction",
	}

	CliChainID = cli.StringFlag{
		Name:  "chainID",
		Value: "hgs",
		Usage: "chainid",
	}

	CliFromAddr = cli.StringFlag{
		Name:  "from",
		Value: "",
		Usage: "from address",
	}

	CliFromChainID = cli.StringFlag{
		Name:  "fromCoinType",
		Value: "",
		Usage: "hnb/hgs",
	}

	CliFromAmount = cli.Int64Flag{
		Name:  "fromAmount",
		Value: 0,
		Usage: "amount",
	}

	CliToAddr = cli.StringFlag{
		Name:  "to",
		Value: "",
		Usage: "to address",
	}

	CliToChainID = cli.StringFlag{
		Name:  "toCoinType",
		Value: "",
		Usage: "hnb/hgs",
	}

	CliToAmount = cli.Int64Flag{
		Name:  "toAmount",
		Value: 0,
		Usage: "amount",
	}

	CliKeyPath = cli.StringFlag{
		Name:  "keyPath",
		Value: "./node.key",
		Usage: "key path",
	}

	//CliQueryType = cli.StringFlag{
	//	Name:  "queryType",
	//	Value: "balance",
	//	Usage: "balance",
	//}

	CliBalanceAddr = cli.StringFlag{
		Name:  "addr",
		Value: "",
		Usage: "Query account balance",
	}

	CliNonce = cli.Int64Flag{
		Name:  "nonce",
		Value: 0,
		Usage: "nonce",
	}

)

var (
	CliSendVoteMsg = cli.StringFlag{
		Name:	"send vote msg",
		Value:	"vote",
		Usage:	"send vote tx to vote for one node",
	}

	CliCandidate = cli.StringFlag{
		Name:  	"candidate",
		Value: 	"",
		Usage: 	"candidate node",
	}

	CliVotingPower = cli.Int64Flag{
		Name:	"votingPower",
		Value:	0,
		Usage:	"node vote power",
	}
	CliVoteEpochNo = cli.Uint64Flag{
		Name:	"voteEpochNo",
		Value:	0,
		Usage:	"voteEpochNo",
	}
)

//var ReadTxPoolLen = cli.Command{
//	Name:   "txpoollen",
//	Usage:  "Get the number of transactions in the tx pool, including two parts queue and pending",
//	Action: GetTxcount,
//	Flags: []cli.Flag{
//		CliChainID,
//	},
//}

// ./querymsg --restport=xxx balance --chainid=xx --addr=xxx
var QueryBalanceCommand = cli.Command{
	Name:   "querybalance",
	Usage:  "query balance by address",
	Action: QueryBalance,
	Flags: []cli.Flag{
		CliChainID,
		CliRest,
		CliBalanceAddr,
		CliKeyPath,
	},
}

var QueryMsgCommand = cli.Command{
	Name:   "querymsg",
	Usage:  "Query msg",
	Action: QueryBalance,
	//Flags: []cli.Flag{
	//	CliRest,
	//},
	Subcommands: []cli.Command{
		QueryBalanceCommand,
	},
}

//  ./start sendmsg tx from=xxx coinType=hnb to=xxx coinType=hgs
//  ./start sendmsg vote
var SendTx = cli.Command{
	Name:   "sendmsg",
	Usage:  "Send the cli interface of the transaction",
	Action: SendMsg,
	Flags: []cli.Flag{
		CliFromAddr,
		CliFromChainID,
		CliToAddr,
		CliToChainID,
		CliRest,
		CliFromAmount,
		CliToAmount,
		CliKeyPath,
		CliNonce,
	},
}

var SendVoteTx = cli.Command{
	Name:	"sendvotemsg",
	Usage:	"send vote tx to vote for one node",
	Action:	SendVoteMsg,
	Flags:	[]cli.Flag{
		//CliFromAddr,
		//CliFromChainID,
		CliRest,
		CliCandidate,
		CliVotingPower,
		CliVoteEpochNo,
		CliKeyPath,
		CliNonce,
	},
}

//func GetTxcount(ctx *cli.Context) error {
//	//chainID := ctx.String(CliRest.Name)
//	//txCount := txpool.TxsLen(chainID)
//	//fmt.Printf(">> the %s txpool len %d\n", chainID, txCount)
//	return nil
//}

func loadKey(path string) (bccsp.Key, error) {
	kp := &msp.KeyPair{}
	sKeyPair, err := msp.Load(path)
	if err != nil {
		return nil, err
	}

	key := new(ecdsa.PrivateKey)
	key.Curve = secp256k1.S256()
	key.D = new(big.Int)
	key.D.SetBytes(sKeyPair.PriKey)
	key.PublicKey.X, key.PublicKey.Y = elliptic.Unmarshal(secp256k1.S256(), sKeyPair.PubKey)
	kp.Scheme = sKeyPair.Scheme
	kp.PubKey = &sw.Ecdsa256K1PublicKey{&key.PublicKey}
	kp.PriKey = &sw.Ecdsa256K1PrivateKey{key}
	return kp.PubKey, nil
}



func SendMsg(ctx *cli.Context) {
	port := ctx.String(CliRest.Name)
	fmt.Println("port:" + port)
	chainID := ctx.String(CliFromChainID.Name)
	toChainID := ctx.String(CliToChainID.Name)

	fromAmount := ctx.Int64(CliFromAmount.Name)
	toAmount := ctx.Int64(CliToAmount.Name)

	//fromAddr := ctx.String(CliFromAddr.Name)
	toAddr := ctx.String(CliToAddr.Name)

	path := ctx.String(CliKeyPath.Name)

	nonce := ctx.Int64(CliNonce.Name)
	isSame := false
	if chainID == toChainID {
		isSame = true
	}

	var payload []byte
	if chainID == txpool.HNB {
		ht := &hnb.HnbTx{}
		if isSame == true {
			ht.TxType = hnb.SAME
			st := &hnb.SameTx{}
			st.OutputAddr = util.HexToByte(toAddr)
			st.Amount = fromAmount
			ht.PayLoad, _ = json.Marshal(st)
			payload, _ = json.Marshal(ht)
		} else {
			ht.TxType = hnb.DIFF
			df := &hnb.DiffTx{}
			df.Amount = toAmount
			df.OutputAddr = util.HexToByte(toAddr)
			df.InAmount = fromAmount
			ht.PayLoad, _ = json.Marshal(df)
			payload, _ = json.Marshal(ht)
		}
	} else if chainID == txpool.HGS {
		ht := &hgs.HgsTx{}
		if isSame == true {
			ht.TxType = hgs.SAME
			st := &hgs.SameTx{}
			st.OutputAddr = util.HexToByte(toAddr)
			st.Amount = fromAmount
			ht.PayLoad, _ = json.Marshal(st)
			payload, _ = json.Marshal(ht)
		} else {
			ht.TxType = hgs.DIFF
			df := &hgs.DiffTx{}
			df.Amount = toAmount
			df.OutputAddr = util.HexToByte(toAddr)
			df.InAmount = fromAmount
			ht.PayLoad, _ = json.Marshal(df)
			payload, _ = json.Marshal(ht)
		}
	} else {
		fmt.Println("chainID invalid")
		return
	}

	key, err := loadKey(path)
	if err != nil {
		fmt.Println("load key ", err.Error())
	}

	msgTx := common.Transaction{}
	address := msp.AccountPubkeyToAddress1(key)
	msgTx.Payload = payload
	msgTx.From = address
	msgTx.ContractName = chainID
	msgTx.NonceValue = uint64(nonce)
	signer := msp.GetSigner()
	msgTx.Txid = signer.Hash(&msgTx)

	err = msp.NewKeyPair().Init("./node.key")
	if err != nil {
		fmt.Println(err)
		return
	}

	msgTxWithSign, err := msp.SignTx(&msgTx, signer)
	if err != nil {
		fmt.Println(err)
		return
	}

	mt,_ := rlp.EncodeToBytes(msgTxWithSign)
	url := "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version:"1.0"}
	jm.Method = "sendRawTransaction"
	var params []interface{}
	params = append(params, util.ToHex(mt))
	jm.Params,_ = json.Marshal(params)
	jmm,_ := json.Marshal(jm)

	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(jmm))

		if err != nil {
			fmt.Println(err)
		}
		result, _ := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(">>" + string(result))
		response.Body.Close()
	}
}

func QueryBalance(ctx *cli.Context) {

	var url string
	port := ctx.String(CliRest.Name)
	fmt.Println("port:" + port)
	chainID := ctx.String(CliChainID.Name)
	fmt.Println("chainID:" + chainID)
	addr := ctx.String(CliBalanceAddr.Name)
	if addr == "" {
		path := ctx.String(CliKeyPath.Name)
		key, err := loadKey(path)
		if err != nil {
			fmt.Println("load key ", err.Error())
			return
		}
		address := msp.AccountPubkeyToAddress1(key)
		addr = util.ByteToHex(address.GetBytes())
	}
	fmt.Println("addr:", addr)
	url = "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version:"1.0"}
	jm.Method = "getBalance"
	var params []interface{}
	params = append(params, chainID, addr)
	jm.Params,_ = json.Marshal(params)
	jmm,_ := json.Marshal(jm)
	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(jmm))

		if err != nil {
			fmt.Println(err)
		}
		result, _ := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(">>" + string(result))
		response.Body.Close()
	}
}

func SendVoteMsg(ctx *cli.Context)  {
	port := ctx.String(CliRest.Name)
	fmt.Println("port:" + port)

	candidate := ctx.String(CliCandidate.Name)

	nonce := ctx.Int64(CliNonce.Name)
	path := ctx.String(CliKeyPath.Name)
	key, err := loadKey(path)
	if err != nil{
		fmt.Println("load key ", err.Error())
	}
	address := msp.AccountPubkeyToAddress1(key)


	votePower := ctx.Int64(CliVotingPower.Name)
	epochNo := ctx.Uint64(CliVoteEpochNo.Name)

	var payload []byte
	ht := &hgs.HgsTx{}
	ht.TxType = hnb.POS_VOTE_TRANSCATION

	voteMsg := &hnb.VoteInfo{}
	voteMsg.FromAddr = address.GetBytes()
	voteMsg.Candidate = msp.StringToByteKey(candidate)
	voteMsg.VotingPower = votePower
	if epochNo == 0 {
		epochNo = dbft.DS.GetCurrentEpochNo()
	}
	voteMsg.VoteEpoch = epochNo

	ht.PayLoad,_ = json.Marshal(voteMsg)
	payload,_ = json.Marshal(ht)

	msgTx := common.Transaction{}
	msgTx.Payload = payload
	msgTx.From = address
	msgTx.ContractName = txpool.HNB
	msgTx.NonceValue = uint64(nonce)
	signer := msp.GetSigner()
	msgTx.Txid = signer.Hash(&msgTx)

	err = msp.NewKeyPair().Init(path)
	if err != nil{
		fmt.Println(err)
		return
	}


	msgTxWithSign, err := msp.SignTx(&msgTx, signer)
	if err != nil {
		fmt.Println(err)
		return
	}


	//f,err := msp.Sender(signer, &msgTx)
	//if err != nil{
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Printf("msg %v %v\n",
	//	util.ByteToHex(address.GetBytes()), util.ByteToHex(f.GetBytes()))



	mt,_ := rlp.EncodeToBytes(msgTxWithSign)

	url := "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version:"1.0"}
	jm.Method = "sendRawTransaction"
	var params []interface{}
	params = append(params, util.ToHex(mt))
	jm.Params,_ = json.Marshal(params)
	jmm,_ := json.Marshal(jm)

	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(jmm))

		if err != nil {
			fmt.Println(err)
		}
		result, _ := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(">>" + string(result))
		response.Body.Close()
	}
}

