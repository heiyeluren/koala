/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Dict list parser & counter
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package main

import (
	"fmt"
	"time"

	"github.com/heiyeluren/koala/utility/logger"
)

// CountMessage .
type CountMessage struct {
	ruleNo   int32
	decision int32
}

// Counter .
type Counter struct {
	allow int64
	deny  int64
}

const (
	// ALLOW .允许
	ALLOW = 1
	// DENY 拒绝
	DENY = 2
)

var (
	// PolicyCounter .
	PolicyCounter map[string]map[int32]*Counter
	// CountTransChannel .
	CountTransChannel chan *CountMessage
)

func init() {
	PolicyCounter = make(map[string]map[int32]*Counter)
	CountTransChannel = make(chan *CountMessage, 1024)
}

// CounterAgent 从 channel 里获取策略结果，然后定期写入日志 channel
func CounterAgent() {

	for {
		// 从channel获取一条countMsg（统计消息）
		countMsg := <-CountTransChannel

		dateString := GetDateString(time.Now())

		if _, OK := PolicyCounter[dateString]; !OK {
			// 将过期的元素（上一时段的）转移存储，并从map摘除
			for k, v := range PolicyCounter {
				recordExpiredCounter(v)
				delete(PolicyCounter, k)
			}
			// 初始化当前时段map
			PolicyCounter[dateString] = make(map[int32]*Counter)
		}
		if _, OK := PolicyCounter[dateString][countMsg.ruleNo]; !OK {
			// 如果map元素不存在，则初始化此元素
			initCounter := new(Counter)
			PolicyCounter[dateString][countMsg.ruleNo] = initCounter
		}

		singleCounter := PolicyCounter[dateString][countMsg.ruleNo]
		switch countMsg.decision {
		case ALLOW:
			singleCounter.allow += 1
		case DENY:
			singleCounter.deny += 1
		default:
		}
	}
}

/**
 * 记录counter数据到日志
 */
func recordExpiredCounter(counters map[int32]*Counter) {
	logHandle := logger.NewLogger(time.Now().Format("20060102") + "_counter")
	for k, v := range counters {
		logMsg := fmt.Sprintf(" rule_no:%d  allow:%d  deny:%d ", k, v.allow, v.deny)
		logHandle.Warning(logMsg)
	}
}

// CounterClient 统计API
func CounterClient(ruleNo int32, deny bool) {
	msg := new(CountMessage)
	msg.ruleNo = ruleNo
	if deny == true {
		msg.decision = DENY
	} else {
		msg.decision = ALLOW
	}

	// 将统计消息，发给channal
	CountTransChannel <- msg
}

// GetDateString .
func GetDateString(t time.Time) string {
	// 按天划分时段
	tString := t.Format("20060102")
	return tString
}

// GetCurrentCounters .
func GetCurrentCounters() map[int32]*Counter {
	dateString := GetDateString(time.Now())
	return PolicyCounter[dateString]
}
