package rpc

type TransactionLogs struct {
	Address          string   `json:"address"`
	BlockHash        string   `json:"blockHash"`
	BlockNumber      string   `json:"blockNumber"`
	Data             string   `json:"data"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
	Topics           []string `json:"topics"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
}

type TrxReceiptData struct {
	BlockHash         string            `json:"blockHash"`
	BlockNumber       string            `json:"blockNumber"`
	ContractAddress   string            `json:"contractAddress"`
	CumulativeGasUsed string            `json:"cumulativeGasUsed"`
	EffectiveGasPrice string            `json:"effectiveGasPrice"`
	From              string            `json:"from"`
	GasUsed           string            `json:"gasUsed"`
	Logs              []TransactionLogs `json:"logs"`
	LogsBloom         string            `json:"logsBloom"`
	Status            string            `json:"status"`
	To                string            `json:"to"`
	TransactionHash   string            `json:"transactionHash"`
	TransactionIndex  string            `json:"transactionIndex"`
	Type              string            `json:"type"`
}
type TraceTransactionData struct {
	Gas         int         `json:"gas"`
	Failed      bool        `json:"failed"`
	ReturnValue string      `json:"returnValue"`
	StructLogs  []StructLog `json:"structLogs"`
}

type StructLog struct {
	Pc      int      `json:"pc"`
	Op      string   `json:"op"`
	Gas     int      `json:"gas"`
	GasCost int      `json:"gasCost"`
	Depth   int      `json:"depth"`
	Stack   []string `json:"stack"`
	Memory  []string `json:"memory"`
}
