package db

import (
	"math/big"
	"scroll-l2-indexer/src/rpc"
)

type UnHandledTransaction struct {
	Hash        string
	BlockNumber uint64
	BlockTime   int64
	Value       float64
	GasLimit    int64
	InputData   string
	BlockHash   string
}

type HandledTransaction struct {
	Hash           string
	Status         uint64
	ErrorInfo      string
	From           string
	To             string
	Fee            uint64
	GasUsed        uint64
	Handled        bool
	CreateContract []CreateContract
	Internal       []InternalTransaction
	TokenTransfer  []TokenTransaction
	Logs           []rpc.TransactionLogs
}

type InternalTransaction struct {
	BlockHash             string
	BlockNumber           uint64
	ParentTransactionHash string
	From                  string
	To                    string
	Value                 string
	TypeTraceAddress      string
	GasLimit              string
	Op                    string
}

type BalanceChange struct {
	Address      string
	Contract     string
	ContractType int
	TokenId      string
	Value        *big.Int
}

type CreateContract struct {
	ContractAddress string
	CreateTxHash    string
	Creator         string
	ByteCode        string
}

type TokenTransaction struct {
	TransactionHash string
	LogIndex        string
	Contract        string
	TokenType       int
	Value           string
	TokenId         string
	From            string
	To              string
	MethodId        string
	BlockHash       string
	BlockTime       int64
}
type ContractNeedInit struct {
	Contract  string
	TokenType int
}

type UpdateContractObject struct {
	ContractAddress string
	ContractType    int
	Symbol          string
	Decimals        int
	TotalSupply     string
	Name            string
}

type TokenBalanceChange struct {
	Address      string
	Contract     string
	ContractType int
	TokenId      string
	Value        *big.Int
	TxHash       string
	LogIndex     int
}
