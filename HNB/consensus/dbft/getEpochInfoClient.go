package dbft

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"io/ioutil"
	"net/http"
	"strconv"
)

type EpochInfo struct {
	EpochNo     uint64   `json:"epoch_no"`
	WitnessList []string `json:"witness_list"`
}
type RetCode string

type RestQueryResult struct {
	RestBase
	Data interface{} `json:"data,omitempty"`
}

type RestBase struct {
	Code    RetCode     `json:"code,omitempty"`
	Message interface{} `json:"msg,omitempty"`
}

func GetEpochInfo(candidateEpochNo uint64) []string {
	client := NewConn()
	if client == nil {
		return nil
	}
	url := config.Config.EpochServer.EpochServerPath + "/getepochwitnesslist/" + strconv.FormatUint(candidateEpochNo, 10)
	fmt.Println("url ", url)
	resp, _ := client.Get(url)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "http Get error %s", err.Error())
		return nil
	}
	result := new(RestQueryResult)
	err = json.Unmarshal(body, &result)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "Unmarshal error %s", err.Error())
		return nil
	}
	if result.Code != "0000" {
		ConsLog.Errorf(LOGTABLE_DBFT, "net 0000 %s", result.Message)
		return nil
	}
	return result.Data.([]string)
}

func NewConn() *http.Client {
	pool := x509.NewCertPool()
	caCertPath := config.Config.EpochServer.CaPath
	fmt.Println(caCertPath)
	caCrt, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		fmt.Println("ReadFile err:", err)
		return nil
	}
	//fmt.Println(caCrt)
	pool.AppendCertsFromPEM(caCrt)
	//pool.AddCert(caCrt)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: "peer",
			RootCAs:    pool,
			//InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}

	return client
}
