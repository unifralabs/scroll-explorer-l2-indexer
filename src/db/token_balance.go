package db

import (
	"database/sql"
	"fmt"
	"scroll-l2-indexer/src/util"
)

func GetTokenBalanceChanged(db *sql.DB) ([]*TokenBalanceChange, error) {
	sqlStatement := `
		SELECT
			tt."from" AS "Address",
			tt.contract AS "Contract",
			tt."tokenType" AS "ContractType",
			tt."tokenId" AS "TokenId",
			tt."transactionHash" AS "TxHash",
			tt."logIndex" AS "LogIndex"
		FROM "tokenTransfer" tt
		WHERE NOT EXISTS (
			SELECT 1
			FROM "tokenBalanceChangeHandled" tth
			WHERE tt."transactionHash" = tth.txhash AND tt."logIndex" = tth."logIndex"
		)
		limit 500;
	`
	rows, err := db.Query(sqlStatement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*TokenBalanceChange, 0)
	for rows.Next() {
		var reHandleBalanceChange TokenBalanceChange
		err = rows.Scan(
			&reHandleBalanceChange.Address,
			&reHandleBalanceChange.Contract,
			&reHandleBalanceChange.ContractType,
			&reHandleBalanceChange.TokenId,
			&reHandleBalanceChange.TxHash,
			&reHandleBalanceChange.LogIndex,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, &reHandleBalanceChange)
	}
	return result, nil
}

func UpdateTokenBalance(db *sql.DB, datas []*TokenBalanceChange) error {
	begin, err := db.Begin()
	if err != nil {
		fmt.Println("UpdateTokenBalance", err)
		return nil
	}

	insert, err := db.Prepare(
		`INSERT INTO public."accountBalance"("address","contract","value","tokenId") 
				VALUES ($1,$2,$3,$4) on conflict (address, contract,"tokenId") do update set "value"=$3`,
	)
	if err != nil {
		fmt.Println("UpdateEthBalance prepare insert failed:", err)
		return nil
	}

	insertTmp, err := db.Prepare(
		`insert into public."tokenBalanceChangeHandled"("txhash", "logIndex") 
				values ($1, $2) on conflict do nothing`,
	)
	if err != nil {
		fmt.Println("UpdateEthBalance prepare insertTmp failed:", err)
		return nil
	}

	defer insert.Close()

	for _, c := range datas {
		_, err = insert.Exec(
			util.ToLower(c.Address),
			c.Contract,
			c.Value.String(),
			c.TokenId,
		)
		if err != nil {
			fmt.Println("UpdateEthBalance bal", err)
			begin.Rollback()
			return err
		}
		_, err = insertTmp.Exec(c.TxHash, c.LogIndex)
		if err != nil {
			fmt.Println("UpdateEthBalance tmp", err)
			begin.Rollback()
			return err
		}
	}

	err = begin.Commit()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return nil
}
