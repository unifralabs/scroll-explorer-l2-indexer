package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"scroll-l2-indexer/src/config"
	"scroll-l2-indexer/src/db"
	"scroll-l2-indexer/src/rpc"
	"scroll-l2-indexer/src/util"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	_ "github.com/ethereum/go-ethereum/rlp"

	_ "io/ioutil"
	"math/big"
	_ "net/http"
	"os"
	_ "os"
	"os/signal"
	"strconv"
	"strings"
	_ "strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/core/vm"
	_ "github.com/lib/pq"
)

type callStackFrame struct {
	op             vm.OpCode
	accountAddress common.Address
	transfers      []*valueTransfer
}
type valueTransfer struct {
	depth           int
	transactionHash string
	src             common.Address
	srcBalance      *big.Int
	dest            common.Address
	destBalance     *big.Int
	value           *big.Int
	kind            string
}
type HandleTraceTransaction struct {
	stack []*callStackFrame
}

func collectBlock(heightChan chan int) {
	for {
		onceHeight := <-heightChan
		//fmt.Println("collecting", onceHeight, "chan", len(heightChan))
		block, err := rpc.EthClient.BlockByNumber(context.Background(), big.NewInt(int64(onceHeight)))
		if err != nil {
			fmt.Println("req block body err, height", onceHeight, err)
			continue
		}

		err = db.InsertRawBlockBody(session, block)
		if err != nil {
			fmt.Println("inerset raw block err height:", onceHeight, err)
			continue
		}

		if onceHeight%100 == 0 {
			fmt.Println("collecting block height (sampled):", onceHeight)
		}
		util.MetricHandleBlk()
	}
}

func handleTx(txsCh chan db.UnHandledTransaction) {

	for {
		unHandledTx := <-txsCh
		//fmt.Println(time.Now(), unHandledTx.Hash)
		handledTx := db.HandledTransaction{}

		// save tx
		res, err := rpc.Call("eth_getTransactionReceipt", &[]interface{}{unHandledTx.Hash})
		if err != nil {
			fmt.Println("eth_getTransactionReceipt err", err)
			continue
		}
		resJsonStr, err := res.MarshalJSON()
		if err != nil {
			fmt.Println("eth_getTransactionReceipt res.MarshalJSON() err", err)
			continue
		}
		var receipt rpc.TrxReceiptData
		err = json.Unmarshal(resJsonStr, &receipt)
		if err != nil {
			fmt.Println("eth_getTransactionReceipt json.Unmarshal err", err)
			continue
		}

		// calcualte fee
		gasP, ok1 := big.NewInt(0).SetString(receipt.EffectiveGasPrice, 0)
		gasU, ok2 := big.NewInt(0).SetString(receipt.CumulativeGasUsed, 0)
		if !ok2 || !ok1 {
			fmt.Println("calc fee err", unHandledTx.Hash, receipt, receipt.EffectiveGasPrice, receipt.CumulativeGasUsed)
			continue
		}
		fee := big.NewInt(0).Mul(gasP, gasU)

		// transaction exec failed try to get reason, collect error info
		var errorInfo string
		if receipt.Status != "0x1" {
			callMap := make(map[string]string, 0)
			callMap["from"] = receipt.From
			callMap["to"] = receipt.To
			callMap["gas"] = hexutil.EncodeUint64(uint64(unHandledTx.GasLimit))
			callMap["value"] = hexutil.EncodeUint64(uint64(unHandledTx.Value))
			callMap["data"] = unHandledTx.InputData

			// todo not sure here
			_, err := rpc.Call(
				"eth_call", &[]interface{}{
					callMap, hexutil.EncodeUint64(unHandledTx.BlockNumber),
				},
			)
			if err != nil {
				//fmt.Println("eth_call err", err)
				errorInfo = err.Error()
			}
		}

		handledTx.Hash = unHandledTx.Hash
		handledTx.Status, _ = hexutil.DecodeUint64(receipt.Status)
		handledTx.ErrorInfo = errorInfo
		handledTx.From = receipt.From
		handledTx.Fee = fee.Uint64()
		handledTx.GasUsed, _ = hexutil.DecodeUint64(receipt.CumulativeGasUsed)

		// save logs
		handledTx.Logs = receipt.Logs

		// save ContractCreate
		if receipt.ContractAddress != "" {
			contractObj := &db.CreateContract{}
			contractObj.ContractAddress = receipt.ContractAddress
			contractObj.ByteCode = unHandledTx.InputData // todo: bug here, byte code may missmatch
			contractObj.Creator = receipt.From
			contractObj.CreateTxHash = unHandledTx.Hash
			handledTx.To = contractObj.ContractAddress
			handledTx.CreateContract = append(handledTx.CreateContract, *contractObj)
		}

		// token transfer
		if receipt.Status == "0x1" {
			for _, log := range receipt.Logs {
				// 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef  ERC721 transfer topics len 3 data -> 0x
				// 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef ERC20 transfer topics len 2 data ->value
				//ERC 1155
				//event TransferSingle(address indexed _operator, address indexed _from, address indexed _to, uint256 _id, uint256 _value);
				//0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62
				//event TransferBatch(address indexed _operator, address indexed _from, address indexed _to, uint256[] _ids, uint256[] _values);
				//4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb
				if len(log.Topics) >= 3 && log.Topics[0] == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
					var tokenTrx db.TokenTransaction
					if log.Data == "0x" {
						//ERC721
						if len(log.Topics) >= 3 {
							tokenTrx.TokenId = log.Topics[3]
							tokenTrx.TokenType = 2
							tokenTrx.Value = "1"
						}
					} else {
						//ERC20
						if len(log.Topics) >= 3 {
							tokenTrx.TokenId = ""
							tokenTrx.TokenType = 1
							tokenTrx.Value = log.Data
						}
					}
					tokenTrx.From = common.HexToAddress(log.Topics[1]).String()
					tokenTrx.To = common.HexToAddress(log.Topics[2]).String()
					tokenTrx.BlockHash = log.BlockHash
					tokenTrx.BlockTime = unHandledTx.BlockTime
					tokenTrx.MethodId = unHandledTx.InputData[:10]
					tokenTrx.Contract = log.Address
					tokenTrx.LogIndex = log.LogIndex
					tokenTrx.TransactionHash = unHandledTx.Hash

					handledTx.TokenTransfer = append(handledTx.TokenTransfer, tokenTrx)
				}
				//erc1155 Single Transfer
				if log.Topics[0] == "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62" {
					var tokenTrx db.TokenTransaction
					tokenTrx.TokenId = log.Data[2:66]
					tokenTrx.TokenType = 3
					tokenTrx.From = common.HexToAddress(log.Topics[2]).String()
					tokenTrx.To = common.HexToAddress(log.Topics[3]).String()
					tokenTrx.Value = log.Data[66:]
					tokenTrx.BlockHash = log.BlockHash
					tokenTrx.BlockTime = unHandledTx.BlockTime
					tokenTrx.MethodId = unHandledTx.InputData[:10]
					tokenTrx.Contract = log.Address
					tokenTrx.LogIndex = log.LogIndex
					tokenTrx.TransactionHash = unHandledTx.Hash
					handledTx.TokenTransfer = append(handledTx.TokenTransfer, tokenTrx)

				}
				//erc1155 batch Transfer
				if log.Topics[0] == "0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb" {
					firstOffset := util.GetInt(log.Data[:66])*2 + 2
					length := util.GetInt("0x" + log.Data[firstOffset:firstOffset+64])
					for i := int64(0); i < length; i++ {
						var tokenTrx db.TokenTransaction
						tokenTrx.TokenId = log.Data[firstOffset+(i+1)*64 : firstOffset+(i+2)*64]
						tokenTrx.TokenType = 3
						tokenTrx.From = common.HexToAddress(log.Topics[2]).String()
						tokenTrx.To = common.HexToAddress(log.Topics[3]).String()
						tokenTrx.Value = log.Data[firstOffset+(i+2+length)*64 : firstOffset+(i+3+length)*64]
						tokenTrx.BlockHash = log.BlockHash
						tokenTrx.BlockTime = unHandledTx.BlockTime
						tokenTrx.MethodId = unHandledTx.InputData[:10]
						tokenTrx.Contract = log.Address
						tokenTrx.LogIndex = log.LogIndex
						tokenTrx.TransactionHash = unHandledTx.Hash
						handledTx.TokenTransfer = append(handledTx.TokenTransfer, tokenTrx)
					}
				}
			}
		}

		// internal tx
		// todo filter out normal tx, but it's not strictly mean it's a ether transfer
		if receipt.Status == "0x1" && unHandledTx.InputData != "0x" && unHandledTx.InputData != "" {
			traceRes, err := rpc.Call(
				"debug_traceTransaction", &[]interface{}{
					unHandledTx.Hash,
				},
			)
			if err != nil || traceRes == nil {
				fmt.Println("trace err", err)
				continue
			}

			traceBytes, err := traceRes.MarshalJSON()
			if err != nil {
				fmt.Println("trace marshl err", err, handledTx.Hash)
				continue
			}
			var traceObject rpc.TraceTransactionData
			err = json.Unmarshal(traceBytes, &traceObject)
			if err != nil {
				fmt.Println("trace Unmarshal err", err, handledTx.Hash)
				continue
			}
			var handleTrace HandleTraceTransaction
			if receipt.To == "" {
				handleTrace.stack = []*callStackFrame{
					{
						accountAddress: common.HexToAddress(receipt.ContractAddress),
						transfers:      []*valueTransfer{},
					},
				}
			} else {
				handleTrace.stack = []*callStackFrame{
					{
						accountAddress: common.HexToAddress(receipt.To),
						transfers:      []*valueTransfer{},
					},
				}
			}

			for _, op := range traceObject.StructLogs {
				handleTrace.AddStructLog(common.HexToHash(unHandledTx.Hash), op)
			}

			// todo. what hell is this logic, why use stack len here
			if len(handleTrace.stack) > 1 {
				fmt.Printf("Transaction not done: %v frames still on the stack\n", len(handleTrace.stack))
			} else if len(handleTrace.stack) == 1 {
				// Find any unset addresses due to contract creation and set them
				fixupCreationAddresses(handleTrace.stack[0].transfers, common.HexToAddress(receipt.ContractAddress))
				for index, transfer := range handleTrace.stack[0].transfers {
					if transfer.kind == "CREATION" {

						contractObj := &db.CreateContract{}
						contractObj.ContractAddress = transfer.dest.String()
						contractObj.ByteCode = unHandledTx.InputData
						contractObj.Creator = receipt.From
						contractObj.CreateTxHash = unHandledTx.Hash

						handledTx.CreateContract = append(handledTx.CreateContract, *contractObj)

					} else if transfer.kind == "TRANSFER" {
						internalObj := &db.InternalTransaction{}
						internalObj.BlockHash = unHandledTx.BlockHash
						internalObj.BlockNumber = unHandledTx.BlockNumber
						internalObj.From = transfer.src.String()
						internalObj.To = transfer.dest.String()
						internalObj.Value = transfer.value.String()
						internalObj.ParentTransactionHash = unHandledTx.Hash
						internalObj.Op = "CALL"
						internalObj.TypeTraceAddress = "CALL_" + strconv.Itoa(index)

						handledTx.Internal = append(handledTx.Internal, *internalObj)
					}
				}

			}
		}
		// you can metrics execution time here
		//util.MeasureExecutionTime(db.UpdateForHandledTransaction, session, handledTx)
		err = db.UpdateForHandledTransaction(session, handledTx)
		if err != nil {
			fmt.Println("handle tx err", err)
			continue
		}

		util.MetricHandleTx()

	}
}

func newTransfer(depth int, txHash common.Hash, src, dest common.Address,
	value *big.Int, kind string) *valueTransfer {
	return &valueTransfer{
		depth:           depth,
		transactionHash: txHash.String(),
		src:             src,
		dest:            dest,
		value:           value,
		kind:            kind,
	}
}

func fixupCreationAddresses(transfers []*valueTransfer, address common.Address) {
	for _, transfer := range transfers {
		if transfer.src == (common.Address{}) {
			transfer.src = address
		} else if transfer.dest == (common.Address{}) {
			transfer.dest = address
		}
	}
}

func (self *HandleTraceTransaction) AddStructLog(txHash common.Hash, entry rpc.StructLog) {
	//log.Printf("Depth: %v, Op: %v", entry.Depth, entry.Op)
	// If an error occurred (eg, out of gas), discard the current stack frame
	//if entry.Err != nil {
	//	self.stack = self.stack[:len(self.stack) - 1]
	//	if len(self.stack) == 0 {
	//		self.err = entry.Err
	//	}
	//	return
	//}

	// If we just returned from a call
	if entry.Depth == len(self.stack)-1 {
		returnFrame := self.stack[len(self.stack)-1]
		self.stack = self.stack[:len(self.stack)-1]
		topFrame := self.stack[len(self.stack)-1]

		if returnFrame.op == vm.CREATE || returnFrame.op == vm.CREATE2 {
			// Now we know our new address, fill it in everywhere.
			topFrame.accountAddress = common.HexToAddress(entry.Stack[len(entry.Stack)-1])
			fixupCreationAddresses(returnFrame.transfers, topFrame.accountAddress)
		}

		// Our call succeded, so add any transfers that happened to the current stack frame
		topFrame.transfers = append(topFrame.transfers, returnFrame.transfers...)
	} else if entry.Depth != len(self.stack) {
		//fmt.Printf("Unexpected stack transition: was %v, now %v\n", len(self.stack), entry.Depth)
	}

	switch entry.Op {
	case "CREATE2": // todo create 2 should support, need test
		fallthrough
	case "CREATE":
		// CREATE adds a frame to the stack, but we don't know their address yet - we'll fill it in
		// when the call returns.
		value := big.NewInt(0)
		value, _ = value.SetString(entry.Stack[len(entry.Stack)-1][2:], 16)
		src := self.stack[len(self.stack)-1].accountAddress
		var transfers []*valueTransfer
		//if value.Cmp(big.NewInt(0)) != 0 {
		transfers = []*valueTransfer{
			newTransfer(
				len(self.stack), txHash, src, common.Address{},
				value, "CREATION",
			),
		}
		//}
		frame := &callStackFrame{
			op:             vm.StringToOp(entry.Op),
			accountAddress: common.Address{},
			transfers:      transfers,
		}

		self.stack = append(self.stack, frame)
	case "CALL":
		// CALL adds a frame to the stack with the target address and value

		value, _ := new(big.Int).SetString(entry.Stack[len(entry.Stack)-3][2:], 16)

		dest := common.HexToAddress(entry.Stack[len(entry.Stack)-2])

		var transfers []*valueTransfer

		if value.Cmp(big.NewInt(0)) != 0 {
			big.NewInt(0)
			src := self.stack[len(self.stack)-1].accountAddress
			transfers = append(
				transfers,
				newTransfer(
					len(self.stack), txHash, src, dest, value,
					"TRANSFER",
				),
			)
		}

		frame := &callStackFrame{
			op:             vm.StringToOp(entry.Op),
			accountAddress: dest,
			transfers:      transfers,
		}
		self.stack = append(self.stack, frame)
	case "STATICCALL":
		//fmt.Println(entry)
		fallthrough

	case "CALLCODE":
		fallthrough

	case "DELEGATECALL":
		// CALLCODE and DELEGATECALL don't transfer value or change the from address, but do create
		// a separate failure domain.
		frame := &callStackFrame{
			op:             vm.StringToOp(entry.Op),
			accountAddress: self.stack[len(self.stack)-1].accountAddress,
		}
		self.stack = append(self.stack, frame)

	}

}

func queryBalance(address string, contract string, contractType int, tokenId string) *big.Int {
	if contractType == 0 {
		balance, err := rpc.EthClient.BalanceAt(context.Background(), common.HexToAddress(address), nil)
		if err != nil {
			fmt.Println("get balance err", err)
			return big.NewInt(0)
		}
		return balance
	}

	if contractType == 1 {
		callData := fmt.Sprintf("0x70a08231%064s", strings.TrimPrefix(address, "0x"))
		callRes := rpc.EthCall(contract, callData)
		return util.GetBigInt(callRes, 16)
	}

	if contractType == 2 {
		if strings.HasPrefix(tokenId, "0x") {
			tokenId = tokenId[2:]
		}
		callData := fmt.Sprintf("0x6352211e%064s", tokenId)
		callRes := rpc.EthCall(contract, callData)
		addr := common.HexToAddress(callRes)
		if strings.ToLower(addr.String()) == strings.ToLower(address) {
			return big.NewInt(1)
		}
		return big.NewInt(0)
	}

	if contractType == 3 {
		if strings.ToLower(address) == "0x0000000000000000000000000000000000000000" {
			return big.NewInt(0)
		}
		if strings.HasPrefix(tokenId, "0x") {
			tokenId = tokenId[2:]
		}
		callData := fmt.Sprintf("0x00fdd58e%064s%064s", strings.TrimPrefix(address, "0x"), tokenId)
		callRes := rpc.EthCall(contract, callData)
		return util.GetBigInt(callRes, 16)
	}

	return big.NewInt(0)
}

func collectErcContract() {
	for {
		contracts := db.GetContractNeedInitList(session)
		var updates []db.UpdateContractObject
		for _, contract := range contracts {
			if contract.TokenType == 1 {
				//erc20
				var oneUpdate db.UpdateContractObject

				oneUpdate.ContractType = 1
				oneUpdate.ContractAddress = contract.Contract
				//symbol

				symbolData := rpc.EthCall(contract.Contract, "0x95d89b41")
				symbol := ""
				if len(symbolData) > 20 {
					symbolLen, err := strconv.ParseInt(symbolData[66:130], 16, 64)
					if err == nil {
						symbolResData, err := hexutil.Decode("0x" + symbolData[130:130+symbolLen*2])
						if err == nil {
							symbol = string(symbolResData)
						}
					}
				}
				oneUpdate.Symbol = symbol
				//name
				nameData := rpc.EthCall(contract.Contract, "0x06fdde03")
				name := ""
				if len(nameData) > 20 {
					nameLen, err := strconv.ParseInt(nameData[66:130], 16, 64)
					if err == nil {
						nameResData, err := hexutil.Decode("0x" + nameData[130:130+nameLen*2])
						if err == nil {
							name = string(nameResData)
						}
					}
				}
				oneUpdate.Name = name
				decimalData := rpc.EthCall(contract.Contract, "0x313ce567")
				decimal := 0
				if len(decimalData) > 20 {
					decimals, err := strconv.ParseInt(decimalData[2:], 16, 64)
					if err == nil {
						decimal = int(decimals)
					}
				}
				oneUpdate.Decimals = decimal
				totalSupplyData := rpc.EthCall(contract.Contract, "0x18160ddd")
				totalSupply := big.NewInt(0)
				if len(totalSupplyData) > 20 {

					totalSupplys, success := totalSupply.SetString(totalSupplyData[2:], 16)
					if !success {
						totalSupply = big.NewInt(0)
					} else {
						totalSupply = totalSupplys
					}

				}
				oneUpdate.TotalSupply = totalSupply.String()

				updates = append(updates, oneUpdate)

			} else if contract.TokenType == 2 {
				//erc721
				var oneUpdate db.UpdateContractObject
				totalSupplyData := rpc.EthCall(contract.Contract, "0x18160ddd")
				totalSupply := big.NewInt(0)
				if len(totalSupplyData) > 20 {

					totalSupplys, success := totalSupply.SetString(totalSupplyData[2:], 16)
					if !success {
						totalSupply = big.NewInt(0)
					} else {
						totalSupply = totalSupplys
					}

				}
				//symbol

				symbolData := rpc.EthCall(contract.Contract, "0x95d89b41")
				symbol := ""
				if len(symbolData) > 20 {
					symbolLen, err := strconv.ParseInt(symbolData[66:130], 16, 64)
					if err == nil {
						symbolResData, err := hexutil.Decode("0x" + symbolData[130:130+symbolLen*2])
						if err == nil {
							symbol = string(symbolResData)
						}
					}
				}
				oneUpdate.Symbol = symbol
				//name
				nameData := rpc.EthCall(contract.Contract, "0x06fdde03")
				name := ""
				if len(nameData) > 20 {
					nameLen, err := strconv.ParseInt(nameData[66:130], 16, 64)
					if err == nil {
						nameResData, err := hexutil.Decode("0x" + nameData[130:130+nameLen*2])
						if err == nil {
							name = string(nameResData)
						}
					}
				}
				oneUpdate.Name = name
				oneUpdate.ContractType = 2
				oneUpdate.ContractAddress = contract.Contract
				oneUpdate.TotalSupply = totalSupply.String()
				updates = append(updates, oneUpdate)
			} else if contract.TokenType == 3 {
				//erc1155
				var oneUpdate db.UpdateContractObject
				//symbol
				symbolData := rpc.EthCall(contract.Contract, "0x95d89b41")
				symbol := ""
				if len(symbolData) > 20 {
					symbolLen, err := strconv.ParseInt(symbolData[66:130], 16, 64)
					if err == nil {
						symbolResData, err := hexutil.Decode("0x" + symbolData[130:130+symbolLen*2])
						if err == nil {
							symbol = string(symbolResData)
						}
					}
				}
				oneUpdate.Symbol = symbol
				//name
				nameData := rpc.EthCall(contract.Contract, "0x06fdde03")
				name := ""
				if len(nameData) > 20 {
					nameLen, err := strconv.ParseInt(nameData[66:130], 16, 64)
					if err == nil {
						nameResData, err := hexutil.Decode("0x" + nameData[130:130+nameLen*2])
						if err == nil {
							name = string(nameResData)
						}
					}
				}
				oneUpdate.Name = name
				oneUpdate.ContractType = 3
				oneUpdate.ContractAddress = contract.Contract
				updates = append(updates, oneUpdate)
			}
		}
		db.UpdateManyContractType(session, &updates)
		time.Sleep(10 * time.Second)
	}
}

func updateEthBalance() {
	for {
		maxId, changeDatas := db.GetNeedHandleBalanceAddresses(session)
		if len(changeDatas) > 0 {
			for _, change := range changeDatas {
				change.Value = queryBalance(change.Address, change.Contract, change.ContractType, change.TokenId)
				util.MetricHandleEthBalanceUpdate()
			}
			db.UpdateEthBalance(session, maxId, changeDatas)
		} else {
			time.Sleep(3000 * time.Millisecond)
		}

	}
}

func updateTokenBalance() {
	for {
		changeDatas, err := db.GetTokenBalanceChanged(session)
		if err != nil {
			fmt.Println("updateTokenBalance err", err)
			time.Sleep(5000 * time.Millisecond)
			continue
		}
		if len(changeDatas) > 0 {
			for _, change := range changeDatas {
				change.Value = queryBalance(change.Address, change.Contract, change.ContractType, change.TokenId)
				util.MetricHandleTokenBalanceUpdate()
				time.Sleep(5 * time.Millisecond)
			}
			err := db.UpdateTokenBalance(session, changeDatas)
			if err != nil {
				fmt.Println("UpdateTokenBalance err", err)
				return
			}
		} else {
			time.Sleep(3000 * time.Millisecond)
		}

	}
}

var session *sql.DB

func main() {

	config.LoadConfig()

	var collectThreads = config.RpcServerConfig.Worker["collect"]
	var handleThreads = config.RpcServerConfig.Worker["handle"]

	fmt.Printf("handleThreads:%d, collectThreads:%d\n", handleThreads, collectThreads)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println("exit code get", sig)
		os.Exit(1)
	}()

	session = db.GetDB()

	err := rpc.InitRpcClient(
		config.RpcServerConfig.RpcPool["size"],
		config.RpcServerConfig.RpcPool["timeout"],
		config.RpcServerConfig.SourceDataHost,
	)
	if err != nil {
		fmt.Println("init rpc pool err", err)
	}

	// collect blocks
	collectChan := make(chan int, 200)
	for i := 0; i < collectThreads; i++ {
		go collectBlock(collectChan)
	}
	checkpointHeight := uint64(0)
	go func() {
		for {
			curHeight, err := rpc.EthClient.BlockNumber(context.Background())
			//fmt.Println("curHeight", curHeight)
			if err != nil {
				//fmt.Println("get cur curHeight err", err)
				time.Sleep(time.Second)
				continue
			}

			if checkpointHeight == 0 {
				if curHeight > 1000 {
					checkpointHeight = curHeight - 1000
				} else {
					checkpointHeight = curHeight
				}
			}

			// check forward
			if len(collectChan) <= 5 {
				blocks, err := db.QueryUnsycedBlock(session, curHeight-100, curHeight, 100)
				//fmt.Println("report check forward:", blocks, len(collectChan), curHeight)
				if err != nil {
					fmt.Println("check forward err", err)
					continue
				}
				for _, bn := range blocks {
					collectChan <- int(bn)
				}

			}

			// check back
			if len(collectChan) <= 5 && checkpointHeight > 0 {
				var checkBackStart uint64
				if checkpointHeight > 1000 {
					checkBackStart = checkpointHeight - 1000
				} else {
					checkpointHeight = curHeight
					checkBackStart = curHeight - 1000
				}

				blocks, err := db.QueryUnsycedBlock(session, checkBackStart, checkpointHeight, 200)
				//fmt.Println("report check back:", blocks, checkBackStart, checkpointHeight, len(collectChan))
				if err != nil {
					fmt.Println("check back err", err)
					continue
				}
				for _, bn := range blocks {
					collectChan <- int(bn)
				}
				checkpointHeight = checkBackStart
			}

			time.Sleep(1000 * time.Millisecond)

		}
	}()

	// handle txs
	handleTxCh := make(chan db.UnHandledTransaction, 200)
	for i := 0; i < handleThreads; i++ {
		go handleTx(handleTxCh)
	}

	go func() {
		for {
			perQuery := 180
			if len(handleTxCh) < 10 {
				txs, err := db.QueryUnHandledTxs(session, perQuery)
				fmt.Println("check txs:", len(txs))
				if err != nil {
					fmt.Println("QueryUnHandledTxs err", err)
					continue
				}
				time.Sleep(5000 * time.Millisecond)
				for _, tx := range txs {
					handleTxCh <- tx
				}
				time.Sleep(300 * time.Millisecond)
			} else {
				time.Sleep(3 * time.Second)
			}
		}
	}()

	go collectErcContract()
	go updateEthBalance()
	go updateTokenBalance()

	for {
		select {}
	}

}
