package util

import (
	"fmt"
	"sync/atomic"
	"time"
)

type Metric struct {
	cnt       int32
	startTime time.Time
}

var (
	tx                 Metric
	blk                Metric
	ethBalanceUpdate   Metric
	tokenBalanceUpdate Metric
)

func metricHandle(metric *Metric, typeName string) {
	if atomic.LoadInt32(&metric.cnt) == 0 {
		metric.startTime = time.Now()
	}
	atomic.AddInt32(&metric.cnt, 1)
	if atomic.LoadInt32(&metric.cnt) >= 30 {
		fmt.Printf(
			"%s %s per second: %f\n",
			time.Now().String(),
			typeName,
			float64(atomic.LoadInt32(&metric.cnt))/time.Since(metric.startTime).Seconds(),
		)
		atomic.StoreInt32(&metric.cnt, 0)
		metric.startTime = time.Now()
	}
}

func MetricHandleTx() {
	metricHandle(&tx, "tx")
}

func MetricHandleBlk() {
	metricHandle(&blk, "blk")
}

func MetricHandleEthBalanceUpdate() {
	metricHandle(&ethBalanceUpdate, "eth_balance_update")
}

func MetricHandleTokenBalanceUpdate() {
	metricHandle(&tokenBalanceUpdate, "token_balance_update")
}
