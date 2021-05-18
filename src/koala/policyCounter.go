package main

import (
    "fmt"
    "time"
    "utility/logger"
)

type CountMessage struct {
    rule_no  int32
    decision int32
}

type Counter struct {
    allow int64
    deny  int64
}

const (
    ALLOW = 1
    DENY  = 2
)

var (
    PolicyCounter     map[string]map[int32]*Counter
    CountTransChannal chan *CountMessage
)

func init() {
    PolicyCounter = make(map[string]map[int32]*Counter)
    CountTransChannal = make(chan *CountMessage, 1024)
}

// 从 channal 里获取策略结果，然后定期写入日志
func CounterAgent() {

    for {
        // 从channal获取一条countMsg（统计消息）
        countMsg := <-CountTransChannal

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
        if _, OK := PolicyCounter[dateString][countMsg.rule_no]; !OK {
            // 如果map元素不存在，则初始化此元素
            initCounter := new(Counter)
            PolicyCounter[dateString][countMsg.rule_no] = initCounter
        }

        var singleCounter *Counter
        singleCounter = PolicyCounter[dateString][countMsg.rule_no]
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

/**
 * 统计API
 */
func CounterClient(ruleNo int32, deny bool) {
    msg := new(CountMessage)
    msg.rule_no = ruleNo
    if deny == true {
        msg.decision = DENY
    } else {
        msg.decision = ALLOW
    }

    // 将统计消息，发给channal
    CountTransChannal <- msg
}

func GetDateString(t time.Time) string {
    // 按天划分时段
    tString := t.Format("20060102")
    return tString
}

func GetCurrentCounters() map[int32]*Counter {
    dateString := GetDateString(time.Now())
    return PolicyCounter[dateString]
}
