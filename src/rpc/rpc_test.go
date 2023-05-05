package rpc

import (
	"context"
	"fmt"
	"github.com/bitly/go-simplejson"
	"testing"
)

func TestRPCInit(t *testing.T) {
	err := InitRpcClient(10, 10, "https://alpha-rpc.scroll.io/l2")
	fmt.Println(err)
	var block map[string]interface{}
	err = RpcClient.Call(&block, "eth_getBlockByNumber", "0x11", true)
	if err != nil {
		fmt.Println("Failed to retrieve block:", err)
	}
	fmt.Println(block)
	json := simplejson.New()
	json.SetPath([]string{}, block)
	fmt.Println(json)
}

func TestCallBlockNumber(t *testing.T) {
	err := InitRpcClient(10, 10, "https://alpha-rpc.scroll.io/l2")
	fmt.Println(err)
	param := make([]interface{}, 0)
	json_data, err := Call("eth_blockNumber", &param)
	fmt.Println(json_data)
	fmt.Println(err)
}

func TestRPCCall(t *testing.T) {
	err := InitRpcClient(10, 10, "https://alpha-rpc.scroll.io/l2")
	fmt.Println(err)
	args := []interface{}{"0x11", true}
	data, err := Call("eth_getBlockByNumber", &args)
	if err != nil {
		fmt.Println("Failed to retrieve block:", err)
	}
	fmt.Println(data)
}

func TestCallBlockNumberWithEthClient(t *testing.T) {
	err := InitRpcClient(10, 10, "https://alpha-rpc.scroll.io/l2")
	res, err := EthClient.BlockNumber(context.Background())
	fmt.Println(res)
	fmt.Println(err)
}

func TestGetBlock(t *testing.T) {
	err := InitRpcClient(10, 10, "https://alpha-rpc.scroll.io/l2")
	fmt.Println(err)
	param := make([]interface{}, 0)
	param = append(param, "0x111")
	param = append(param, true)
	json_data, err := Call("eth_getBlockByNumber", &param)
	fmt.Println(json_data)
	fmt.Println(err)
}
