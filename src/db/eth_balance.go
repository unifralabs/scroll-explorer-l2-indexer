package db

import (
	"database/sql"
	"fmt"
	"scroll-l2-indexer/src/util"
)

func UpdateEthBalance(db *sql.DB, maxId int64, datas []*BalanceChange) {
	insert, err := db.Prepare(
		`INSERT INTO public."accountBalance"("address","contract","value","tokenId") 
						VALUES ($1,$2,$3,$4) on conflict (address, contract,"tokenId") do update set "value"=$3`,
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer insert.Close()
	begin, err := db.Begin()

	for _, one_data := range datas {
		_, err = begin.Stmt(insert).Exec(
			util.ToLower(one_data.Address),
			one_data.Contract,
			one_data.Value.String(),
			one_data.TokenId,
		)
		if err != nil {
			fmt.Println("UpdateAccountBalance", err)
			begin.Rollback()
			return
		}
	}

	err = begin.Commit()
	if err != nil {
		fmt.Println(err)
		return
	}
	sqlStatement := `delete from public."balanceChange" where "id"<=$1;`
	_, err = db.Exec(sqlStatement, maxId)
	if err != nil {
		fmt.Println("UpdateAccountBalance delete datas", err.Error())
	}
}

func GetNeedHandleBalanceAddresses(db *sql.DB) (int64, []*BalanceChange) {
	sqlStatement := `
			WITH first_50_rows AS (
			    SELECT "id", "address", "contract", "contractType", "tokenId"
			    FROM public."balanceChange"
			    ORDER BY "id"
			    LIMIT 1000
			),
			     distinct_rows AS (
			         SELECT DISTINCT ON ("address", "contract", "tokenId") "id", "address", "contract", "contractType", "tokenId"
			         FROM first_50_rows
			         ORDER BY "address", "contract", "tokenId", "id" desc 
			     )
			SELECT * FROM distinct_rows ORDER BY "id";
	`
	rows, err := db.Query(sqlStatement)
	if err != nil {
		return 0, []*BalanceChange{}
	}

	defer rows.Close()

	res := make([]*BalanceChange, 0, 100)
	maxId := int64(0)
	id := int64(0)
	for rows.Next() {
		var balanceChange BalanceChange
		err = rows.Scan(
			&id,
			&balanceChange.Address,
			&balanceChange.Contract,
			&balanceChange.ContractType,
			&balanceChange.TokenId,
		)
		if err != nil {
			fmt.Println(err)
			return 0, []*BalanceChange{}
		}
		if id > maxId {
			maxId = id
		}

		res = append(res, &balanceChange)
	}
	return maxId, res
}
