package util

import (
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"math/big"
	"strconv"
	"strings"
)

func JsonObjectToString(data interface{}) string {
	datas, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(datas)
}

func ToLower(s string) string {
	return strings.ToLower(s)
}

func IsEmptyJson(jsonObj *simplejson.Json) bool {
	jsonMap, err := jsonObj.Map()
	if err != nil {
		return false
	}
	return len(jsonMap) == 0
}

func GetBigInt(str string, base int) *big.Int {
	if str == "" {
		return big.NewInt(0)
	}
	if str == "0x" {
		return big.NewInt(0)
	}
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}
	res, _ := big.NewInt(0).SetString(str, base)

	return res
}

func GetBigIntString(str string, base int) string {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}
	if str == "" {
		return "0"
	}
	res, _ := big.NewInt(0).SetString(str, base)
	return res.String()
}

func GetInt(str string) int64 {
	if strings.HasPrefix(str, "0x") {
		value, _ := strconv.ParseInt(str[2:], 16, 32)
		return value
	} else {
		value, _ := strconv.ParseInt(str, 10, 32)
		return value
	}
}
