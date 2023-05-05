# Scroll Explorer L2 Indexer

Scroll Explorer L2 Indexer is an indexer for indexing data from scroll L2 network to pg database.

# Example config

```json
{
  "SourceDataHost": "https://alpha-rpc.scroll.io/l2",
  "PosgresqlConfig": {
    "dbname": "",
    "host": "0.0.0.0",
    "password": "",
    "port": "5432",
    "user": "postgres"
  },
  "Worker": {
    "handle": 2,
    "collect": 2
  },
  "RpcPool": {
    "size": 10,
    "timeout": 10
  }
}
```

# How To Start

```
go mod tidy
go mod download
go build main.go
```
