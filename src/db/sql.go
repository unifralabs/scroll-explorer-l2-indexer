package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	_ "github.com/lib/pq"
	"scroll-l2-indexer/src/config"
	"scroll-l2-indexer/src/rpc"
	"scroll-l2-indexer/src/util"
	"strings"
	"time"
)

func GetDB() *sql.DB {

	cfg := config.RpcServerConfig.PosgresqlConfig.(map[string]interface{})

	psqlInfo := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg["host"].(string),
		cfg["port"].(string),
		cfg["user"].(string),
		cfg["password"].(string),
		cfg["dbname"].(string),
	)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		fmt.Printf(err.Error())
		return nil
	}
	err = db.Ping()
	if err != nil {
		fmt.Printf(err.Error())
		return nil
	}
	fmt.Println("successfull connected!")
	db.SetMaxOpenConns(9)
	return db
}

func GetConfigHeight(db *sql.DB) int {
	sqlStatement := `SELECT "value" FROM public.config where "key"='trxHeight';`
	row := db.QueryRow(sqlStatement)
	var height int
	err := row.Scan(&height)
	switch err {
	case sql.ErrNoRows:
		sqlStatement := `INSERT INTO public.config("key","value") VALUES ('trxHeight',$1);`
		_, err = db.Exec(sqlStatement, 0)
		return 0
	case nil:
		return height
	default:
		panic(err)
	}
}

func GetContractNeedInitList(db *sql.DB) []ContractNeedInit {
	sqlStatement := `
				SELECT DISTINCT t."contract", t."tokenType"
				FROM "tokenTransfer" t
				WHERE NOT EXISTS (
				    SELECT 1
				    FROM "contract" c
				    WHERE t."contract" = c."contractAddress" AND c."contractType" IS NOT NULL
				)
				limit 100;`
	rows, err := db.Query(sqlStatement)
	if err != nil {
		return []ContractNeedInit{}
	}
	var contractNeedInit ContractNeedInit
	defer rows.Close()

	res := make([]ContractNeedInit, 0, 100)
	for rows.Next() {
		err = rows.Scan(&contractNeedInit.Contract, &contractNeedInit.TokenType)
		if err != nil {
			fmt.Println(err)
			return []ContractNeedInit{}
		}

		res = append(res, contractNeedInit)
	}
	return res
}

func UpdateManyContractType(db *sql.DB, datas *[]UpdateContractObject) {

	update1, err := db.Prepare(`insert into public."contract"("contractAddress","symbol","contractType","decimals","totalSupply","name") values($6,$1,$2,$3,$4,$5) on conflict("contractAddress") do update set "symbol"=$1,"contractType"=$2,"decimals"=$3,"totalSupply"=$4,"name"=$5;`)
	if err != nil {
		fmt.Println("1", err)
		return
	}
	update2, err := db.Prepare(`insert into public."contract"("contractAddress","symbol","contractType","totalSupply","name") values($3,$4,$1,$2,$5) on conflict("contractAddress") do update set "contractType"=$1,"totalSupply"=$2,"symbol"=$4,"name"=$5;`)
	if err != nil {
		fmt.Println("2", err)
		return
	}
	update3, err := db.Prepare(`insert into public."contract"("contractAddress","symbol","contractType","name") values($2,$3,$1,$4) on conflict("contractAddress") do update set "contractType"=$1,"symbol"=$3,"name"=$4;`)
	if err != nil {
		fmt.Println("3", err)
		return
	}
	defer update1.Close()
	defer update2.Close()
	defer update3.Close()

	begin, err := db.Begin()

	for _, one_data := range *datas {
		if one_data.ContractType == 1 {
			fmt.Println(
				one_data.Symbol,
				one_data.ContractType,
				one_data.Decimals,
				one_data.TotalSupply,
				one_data.Name,
				one_data.ContractAddress,
			)
			_, err = begin.Stmt(update1).Exec(
				one_data.Symbol,
				one_data.ContractType,
				one_data.Decimals,
				one_data.TotalSupply,
				one_data.Name,
				util.ToLower(one_data.ContractAddress),
			)
			if err != nil {
				fmt.Println("UpdateManyContractType", err)
				begin.Rollback()
				return
			}
		} else if one_data.ContractType == 2 {
			_, err = begin.Stmt(update2).Exec(
				one_data.ContractType,
				one_data.TotalSupply,
				util.ToLower(one_data.ContractAddress),
				one_data.Symbol,
				one_data.Name,
			)
			if err != nil {
				fmt.Println("UpdateManyContractType", err)
				begin.Rollback()
				return
			}
		} else if one_data.ContractType == 3 {
			_, err = begin.Stmt(update3).Exec(
				one_data.ContractType,
				util.ToLower(one_data.ContractAddress),
				one_data.Symbol,
				one_data.Name,
			)
			if err != nil {
				fmt.Println("UpdateManyContractType", err)
				begin.Rollback()
				return
			}
		}
	}

	err = begin.Commit()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func insertManyContractTrx(db *sql.Tx, datas *[]CreateContract) error {
	insert, err := db.Prepare(`INSERT INTO public."contract"("contractAddress","createTxHash",creator,"byteCode") VALUES ($1,$2,$3,$4) on conflict("contractAddress") do nothing`)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer insert.Close()
	for _, one_data := range *datas {

		_, err = insert.Exec(
			util.ToLower(one_data.ContractAddress),
			one_data.CreateTxHash,
			one_data.Creator,
			one_data.ByteCode,
		)
		if err != nil {
			fmt.Println("insertManyContractTrx", err)
			return nil
		}
	}

	return nil
}

func insertManyTrxLogData(db *sql.Tx, datas *[]rpc.TransactionLogs) error {
	insert, err := db.Prepare(`INSERT INTO public."transactionLogs"("transactionHash","logIndex",address,"blockHash","blockNumber",data,removed,topics,"transactionIndex") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict ("transactionHash", "logIndex") do nothing`)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer insert.Close()
	for _, one_data := range *datas {

		_, err = insert.Exec(
			one_data.TransactionHash,
			util.GetBigIntString(one_data.LogIndex, 16),
			util.ToLower(one_data.Address),
			one_data.BlockHash,
			util.GetBigIntString(one_data.BlockNumber, 16),
			one_data.Data,
			one_data.Removed,
			util.JsonObjectToString(one_data.Topics),
			util.GetBigIntString(one_data.TransactionIndex, 16),
		)
		if err != nil {
			fmt.Println("insertManyTrxLogData", err)
			return nil
		}
	}

	return nil
}

func insertManyTokenTrx(db *sql.Tx, datas *[]TokenTransaction) error {
	insert, err := db.Prepare(
		`
		INSERT INTO public."tokenTransfer"("transactionHash","logIndex",contract,"tokenType",
                            value,"tokenId","from","to","methodId","blockHash","blockTime") 
		SELECT $1::varchar, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		WHERE NOT EXISTS (
			SELECT 1
			FROM public."tokenTransfer"
			WHERE "transactionHash" = $1::varchar AND "logIndex" = $2
		)
	`,
	)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer insert.Close()
	for _, one_data := range *datas {

		_, err = insert.Exec(
			one_data.TransactionHash,
			util.GetBigIntString(one_data.LogIndex, 16),
			util.ToLower(one_data.Contract),
			one_data.TokenType,
			util.GetBigIntString(one_data.Value, 16),
			one_data.TokenId,
			util.ToLower(one_data.From),
			util.ToLower(one_data.To),
			one_data.MethodId,
			one_data.BlockHash,
			one_data.BlockTime,
		)
		if err != nil {
			fmt.Println("InsertManyTokenTrx", err)
			return err
		}
	}

	return nil
}

func insertManyInternalTrx(db *sql.Tx, datas *[]InternalTransaction) error {
	insert, err := db.Prepare(
		`INSERT INTO public."internalTransaction"("blockHash","blockNumber","parentTransactionHash","from",
                            "to",value,"typeTraceAddress","op") VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
	)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer insert.Close()

	for _, one_data := range *datas {

		_, err = insert.Exec(
			one_data.BlockHash,
			one_data.BlockNumber,
			one_data.ParentTransactionHash,
			util.ToLower(one_data.From),
			util.ToLower(one_data.To),
			one_data.Value,
			one_data.TypeTraceAddress,
			one_data.Op,
		)
		if err != nil {
			fmt.Println("InsertManyInternalTrx", err)
			return err
		}
	}
	return nil
}

func insertManyBalanceChange(db *sql.Tx, datas *[]BalanceChange) error {
	insert, err := db.Prepare(
		`INSERT INTO public."balanceChange"("address","contract","contractType","tokenId") 
			   VALUES ($1,$2,$3,$4)`,
	)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer insert.Close()
	for _, one_data := range *datas {

		_, err = insert.Exec(
			strings.ToLower(one_data.Address),
			one_data.Contract,
			one_data.ContractType,
			one_data.TokenId,
		)
		if err != nil {
			fmt.Println("InsertManyBalanceDiff", err)
			return nil
		}
	}

	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func restoreBalanceChange(handledTx HandledTransaction) (*[]BalanceChange, error) {
	b := make([]BalanceChange, 0)

	if handledTx.From != "" {
		b = append(
			b, BalanceChange{Address: handledTx.From, ContractType: 0},
		)
	}

	if handledTx.To != "" {
		b = append(
			b, BalanceChange{Address: handledTx.To, ContractType: 0},
		)
	}

	if len(handledTx.Internal) > 0 {
		for _, tx := range handledTx.Internal {
			b = append(
				b, BalanceChange{Address: tx.To, ContractType: 0},
			)
			b = append(
				b, BalanceChange{Address: tx.From, ContractType: 0},
			)
		}

	}

	if len(handledTx.CreateContract) > 0 {
		for _, tx := range handledTx.CreateContract {
			b = append(
				b, BalanceChange{Address: tx.Creator, ContractType: 0},
			)
			b = append(
				b, BalanceChange{Address: tx.ContractAddress, ContractType: 0},
			)
		}
	}

	//if len(handledTx.TokenTransfer) > 0 {
	//	for _, transfer := range handledTx.TokenTransfer {
	//		b = append(
	//			b,
	//			BalanceChange{
	//				Address:      transfer.From,
	//				ContractType: transfer.TokenType,
	//				Contract:     transfer.Contract,
	//				TokenId:      transfer.TokenId,
	//			},
	//		)
	//		b = append(
	//			b,
	//			BalanceChange{
	//				Address:      transfer.To,
	//				ContractType: transfer.TokenType,
	//				Contract:     transfer.Contract,
	//				TokenId:      transfer.TokenId,
	//			},
	//		)
	//	}
	//}
	return &b, nil
}

// UpdateForHandledTransaction example metrics
// explorer_collect/src/db.UpdateForHandledTransaction take: 42 ms
// explorer_collect/src/db.UpdateForHandledTransaction take: 17 ms
// explorer_collect/src/db.UpdateForHandledTransaction take: 11 ms
// explorer_collect/src/db.UpdateForHandledTransaction take: 51 ms
// explorer_collect/src/db.UpdateForHandledTransaction take: 55 ms
func UpdateForHandledTransaction(db *sql.DB, tx HandledTransaction) error {
	begin, err := db.Begin()
	if err != nil {
		return err
	}

	query := `
		UPDATE transaction
		SET
			status = $2,
			"errorInfo" = $3,
			"from" = $4,
			fee = $5,
			"gasUsed" = $6,
			handled = true
		WHERE
			hash = $1;
	`
	_, err = begin.Exec(query, tx.Hash, tx.Status, tx.ErrorInfo, tx.From, tx.Fee, tx.GasUsed)

	if err != nil {
		begin.Rollback()
		return err
	}

	if len(tx.Internal) > 0 {
		err = insertManyInternalTrx(begin, &tx.Internal)
		if err != nil {
			begin.Rollback()
			return err
		}
	}
	if len(tx.TokenTransfer) > 0 {
		err = insertManyTokenTrx(begin, &tx.TokenTransfer)
		if err != nil {
			begin.Rollback()
			return err
		}
	}
	if len(tx.Logs) > 0 {
		err = insertManyTrxLogData(begin, &tx.Logs)
		if err != nil {
			begin.Rollback()
			return err
		}
	}
	if len(tx.CreateContract) > 0 {
		err = insertManyContractTrx(begin, &tx.CreateContract)
		if err != nil {
			begin.Rollback()
			return err
		}
	}

	// balance change itermidiate table
	balanceChanges, err := restoreBalanceChange(tx)
	if err != nil {
		begin.Rollback()
		return err
	}
	err = insertManyBalanceChange(begin, balanceChanges)
	if err != nil {
		begin.Rollback()
		return err
	}

	err = begin.Commit()
	if err != nil {
		return err
	}

	return nil
}

func RemoveAllDataByHash(db *sql.DB, blockHash string) error {
	sqlStatement := `delete from public."internalTransaction" where "blockHash"=$1 `
	_, err := db.Exec(sqlStatement, blockHash)
	if err != nil {
		fmt.Println(err)
	}
	sqlStatement1 := `delete from public."tokenTransfer" where "blockHash"=$1 `
	_, err = db.Exec(sqlStatement1, blockHash)
	if err != nil {
		fmt.Println(err)
	}
	sqlStatement2 := `delete from public."transaction" where "blockHash"=$1 `
	_, err = db.Exec(sqlStatement2, blockHash)
	if err != nil {
		fmt.Println(err)
	}
	sqlStatement3 := `delete from public."transactionLogs" where "blockHash"=$1 `
	_, err = db.Exec(sqlStatement3, blockHash)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

func InsertRawBlockBody(db *sql.DB, block *types.Block) error {

	begin, err := db.Begin()

	// insert blockx
	blockSmt, err := db.Prepare(
		`INSERT INTO block
    ("blockHash","blockNumber","transactionCount","internalTransactionCount",validator,difficult,
    "totalDifficult","blockSize","gasUsed","gasLimit","extraData","parentHash","sha3Uncle",nonce,"blockTime", "Uncles") 
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15, $16) on Conflict("blockHash") do nothing; `,
	)

	var validatorAddr string
	coinbase, err := util.GetSigner(block.Header())
	if err != nil {
		fmt.Println(err)
		validatorAddr = ""
	} else {
		validatorAddr = strings.ToLower(coinbase.String())
	}

	_, err = begin.Stmt(blockSmt).Exec(
		block.Hash().String(),
		block.Number().String(),
		len(block.Transactions()),
		0,
		validatorAddr,
		block.Difficulty().String(),
		0, // todo: set total difficulty to 0 here
		block.Size(),
		block.Header().GasUsed,
		block.Header().GasLimit,
		hexutil.Encode(block.Extra()),
		block.ParentHash().String(),
		block.UncleHash().String(),
		block.Nonce(),
		block.Time(),
		"[]", // uncles always be empty string
	)
	if err != nil {
		return err
	}

	txStmt, err := db.Prepare(
		`INSERT INTO "transaction" (hash, "blockNumber", "blockTime", "to", "value", "gasLimit", "gasPrice", 
                           "transactionType", "maxPriority", "maxFee", "nonce", "inputData", "blockHash", "transactionIndex")
	 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT (hash, "blockHash") DO NOTHING`,
	)
	if err != nil {
		return err
	}

	for txIndex, tx := range block.Transactions() {
		var to string
		if tx.To() != nil {
			to = strings.ToLower(tx.To().String())
		}

		_, err = begin.Stmt(txStmt).Exec(
			tx.Hash().String(),
			block.Number().Int64(),
			block.Time(),
			to,                 // if to is not set, "" in db
			tx.Value().Int64(), // todo overflow
			tx.Gas(),
			tx.GasPrice().Int64(),
			tx.Type(),
			tx.GasTipCap().Int64(),
			tx.GasFeeCap().Int64(), // todo overflow
			tx.Nonce(),
			hexutil.Encode(tx.Data()),
			block.Hash().String(),
			txIndex,
		)
		if err != nil {
			fmt.Println("InsertManyTrxData", err)
			err := begin.Rollback()
			return err
		}
	}

	// Commit the transaction if there's no error
	err = begin.Commit()
	if err != nil {
		fmt.Println("Commit error", err)
		return err
	}

	return nil

}

func QueryUnsycedBlock(db *sql.DB, startBlockHeight uint64, curBlockHeight uint64, limit uint64) ([]uint64, error) {
	stmt := `   WITH available_blocks AS (
				    SELECT generate_series($1::int8, $2::int8) AS block_number
				),
				missing_blocks AS (
				    SELECT block_number
				    FROM available_blocks
				    WHERE NOT EXISTS (
				        SELECT 1
				        FROM block
				        WHERE block."blockNumber" = available_blocks.block_number
				    )
				)
				SELECT block_number
				FROM missing_blocks
				LIMIT $3`
	rows, err := db.Query(stmt, startBlockHeight, curBlockHeight, limit)
	if err != nil {
		return nil, err
	}

	bns := make([]uint64, 0)
	if rows == nil {
		return bns, nil
	}

	for rows.Next() {
		var bn uint64
		err = rows.Scan(&bn)
		if err != nil {
			return nil, err
		}
		bns = append(bns, bn)
	}

	defer rows.Close()
	return bns, nil
}

func QueryUnHandledTxs(db *sql.DB, limit int) ([]UnHandledTransaction, error) {
	query := `SELECT hash, "blockNumber", "blockTime", value, "gasLimit", "inputData", "blockHash" FROM transaction 
			  WHERE handled = false
			  order by "blockNumber" desc
			  limit $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []UnHandledTransaction
	for rows.Next() {
		var tx UnHandledTransaction
		err := rows.Scan(
			&tx.Hash,
			&tx.BlockNumber,
			&tx.BlockTime,
			&tx.Value,
			&tx.GasLimit,
			&tx.InputData,
			&tx.BlockHash,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}
