package db

import (
	"fmt"
	"scroll-l2-indexer/src/config"
	"testing"
)

func TestQueryUnsycedBlock(t *testing.T) {
	config.LoadConfig()
	session := GetDB()
	res, err := QueryUnsycedBlock(session, 1061022-100, 1061022, 100)
	fmt.Println(err)
	fmt.Println(res)
}

func TestQueryTxs(t *testing.T) {
	config.LoadConfig()
	session := GetDB()
	res, err := QueryUnHandledTxs(session, 10)
	fmt.Println(err)
	fmt.Println(res)
}
