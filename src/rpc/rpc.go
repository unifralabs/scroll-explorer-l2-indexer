package rpc

import (
	"errors"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"net/http"
	"scroll-l2-indexer/src/util"
	"time"
)

// RpcClient InitRpcClient A rpc client with connection pooling
var RpcClient *rpc.Client
var EthClient *ethclient.Client

func InitRpcClient(poolSize int, poolTimeout int, nodeURL string) error {
	transport := &http.Transport{
		MaxIdleConnsPerHost:   poolSize,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   time.Duration(poolTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(poolTimeout) * time.Second,
	}
	httpClient := &http.Client{
		Transport: transport,
	}
	client, err := rpc.DialHTTPWithClient(nodeURL, httpClient)
	RpcClient = client
	EthClient = ethclient.NewClient(RpcClient)
	return err
}

func Call(function string, args *[]interface{}) (*simplejson.Json, error) {
	var res interface{}
	err := RpcClient.Call(&res, function, *args...)
	if err != nil {
		return nil, err
	}
	json := simplejson.New()
	json.SetPath([]string{}, res)
	if util.IsEmptyJson(json) {
		return nil, errors.New("empty response")
	}
	return json, nil
}

func EthCall(address string, methodId string) string {
	callMap := make(map[string]string, 0)
	callMap["to"] = address
	callMap["data"] = methodId
	args := make([]interface{}, 0, 2)
	args = append(args, callMap)
	args = append(args, "latest")
	res, err := Call("eth_call", &args)

	if err != nil {
		fmt.Println("eth call err", err, args)
		return "0x"
	}

	resStr, err := res.String()
	if err != nil {
		fmt.Println("eth call json err", err)
		return "0x"
	}

	return resStr
}
