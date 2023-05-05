package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type ServerConfig struct {
	SourceDataHost  string
	PosgresqlConfig interface{}
	SafeBlock       int
	Worker          map[string]int
	RpcPool         map[string]int
}

var RpcServerConfig = ServerConfig{}

func PathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
func LoadConfig() bool {
	if PathExist("l2config.json") {
		data, err := ioutil.ReadFile("l2config.json")
		if err != nil {
			return false
		}
		json.Unmarshal(data, &RpcServerConfig)
		return true
	}
	return false
}
