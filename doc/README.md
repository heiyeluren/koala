## Koala 核心统计频率算法实现


<br />

通用的频率控制服务 Koala，是用 GO 语言开发的后端独立服务。其中有两种控制的频率控制类型分别是

- count （计数型） 
- leak （漏桶型）

这两种频率控制类型都是在指定范围时间 time 内，最多可以访问 count 次数，但究竟有什么区别呢？下面我们来看看他们的实现方式。

### count（计数型）
count 类型直接利用 redis 的 setex() 来计数和设置时间。也就是，当第一次请求过来的时候，创建一个过期时间为 time 的 key ，并设置为 1 ，在接下去的时间内，每次请求过来通过的话 key 就加 1 ，当达到 count的时候，就会禁止访问，直到 key 过期。

具体的实现如下：
（参考 https://github.com/heiyeluren/koala/blob/main/src/koala/koalaRule.go ）


```golang
package main

import (
    "errors"
    "github.com/gomodule/redigo/redis"
    "sort"
    "strconv"
    "strings"
    "time"
)

const (
    // base附加cache key的后缀；在getCacheKey()的key后追加
    BASE_KEY_SUFFIX = "_B"
)

/**
 * rule类型
 */
type KoalaRule struct {
    methord    string //只能为如下四个字符串 count base direct leak
    keys       map[string]KoalaKey
    base       int32
    time       int32
    count      int32
    erase1     int32
    erase2     int32
    result     int32
    returnCode int32
}

/**
 * 浏览；count规则缓存查询、比较
 */
func (k *KoalaRule) countBrowse(cacheKey string) (bool, error) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var err error
    var cacheValue int
    cacheValue, err = redis.Int(redisConn.Do("GET", cacheKey))
    if err == redis.ErrNil {
        // 此 key 不存在，直接返回通过
        return false, nil
    }
    if err != nil {
        return false, err
    }
    //println(cacheKey, " --> value:", cacheValue)
    if k.count == 0 || k.count > int32(cacheValue) {
        return false, nil
    }
    return true, nil
}

/**
 * 更新；count规则缓存更新
 */
func (k *KoalaRule) countUpdate(cacheKey string) error {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var exists int
    var err error
    if exists, err = redis.Int(redisConn.Do("EXISTS", cacheKey)); err != nil {
        return err
    }
    // set new cache
    if exists == 0 {
        var expireTime int32
        // 86400 按照 自然天计算 过期时间
        if k.time == 86400 {
            y, m, d := time.Now().Date()
            loc, _ := time.LoadLocation("Asia/Shanghai")
            dayEnd := time.Date(y, m, d, 23, 59, 59, 0, loc).Unix()
            expireTime = int32(dayEnd - time.Now().Unix())
        } else {
            expireTime = k.time
        }

        if _, err := redis.String(redisConn.Do("SETEX", cacheKey, expireTime, 1)); err != nil {
            return err
        }

        return nil
    }

    // update
    if _, err := redis.Int(redisConn.Do("INCR", cacheKey)); err != nil {
        return err
    }

    return nil
}
```



这种方法实现起来比较简单，但是这可能出现短时间流量暴增问题。比如，某个接口限制是 5 秒只能访问 3 次，在前 1 秒，只访问了一次，在 5 秒快过期的时候，突然访问了 2 次，在 6 秒的时候又访问了 3 次，相当于在 2 秒内访问了 5 次，流量短时间跟预期的比翻了一倍，所以后面添加了**漏桶型**来解决这个问题。


### leak（漏桶型）
漏桶模式是基于漏桶算法，能够平滑网络上的流量，简单的讲就是在过去 time 秒内，访问次数不能超过 count 次，解决 count 流量倍增问题。

漏桶模式可以利用 redis 的 ++list++ 数据结构或 ++zset++ 数据结构来实现。

##### 采用List的数据结构
###### 存储设计和判别条件
采用 redis 的 list 数据结构，实现一种先进先出的队列。

队列的每个元素，存储一个时间戳，记录一次访问的时间。

漏桶大小为 count。

如果第 count 个元素的时间戳，距离当前时间，小于等于 time ，则说明漏桶有“溢出”。

###### 过期元素清除
因为 redis 不能设置 list 里元素过期时间，所以需要手动删除，有两种方法：

- 可以在每次访问后清除队尾多余元素。
- 可以利用 go 协程进行异步处理，不影响速度。

可能出现一个key访问一段时间后突然不访问，导致内存浪费，还需要设置大于 time 的过期时间。

具体实现如下：

```golang
/**
 * leak模式--查询
 *
 */
func (k *KoalaRule) leakBrowse(cacheKey string) (bool, error) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var err error
    var listLen int
    var edgeElement int64
    if listLen, err = redis.Int(redisConn.Do("LLEN", cacheKey)); err != nil {
        return false, err
    }
    if listLen == 0 || listLen <= int(k.count) {
        return false, nil
    }

    defer func() {
        go this.leakClear(cacheKey, listLen)
    }()

    now := time.Now().Unix()
    if edgeElement, err = redis.Int64(redisConn.Do("LINDEX", cacheKey, k.count)); err != nil {
        return false, err
    }
    if int32(now-edgeElement) <= k.time {
        return true, nil
    }
    return false, nil
}

/**
 * leak模式--清理
 * 清理队尾过期多余元素
 */
func (k *KoalaRule) leakClear(cacheKey string, listLen int) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    for listLen > int(k.count+1) {
        if _, err := redis.Int64(redisConn.Do("RPOP", cacheKey)); err != nil {
            return
        }
        listLen--
    }
}

/**
 * leak模式--更新
 */
func (k *KoalaRule) leakUpdate(cacheKey string) error {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    now := time.Now().Unix()
    if _, err := redis.Int(redisConn.Do("LPUSH", cacheKey, now)); err != nil {
        return err
    }
    if _, err := redis.Int(redisConn.Do("EXPIRE", cacheKey, 2*this.time)); err != nil {
        return err
    }
    return nil
}

/**
 * leak模式--反馈
 * 根据指令，减少桶内若干元素
 */
func (k *KoalaRule) leakFeedback(cacheKey string, feedback int) error {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    for feedback > 0 {
        if _, err := redis.Int64(redisConn.Do("LPOP", cacheKey)); err != nil {
            return err
        }
        feedback--
    }
    return nil
}
```

##### 采用Zset的数据结构
###### 存储设计和判别条件
采用 redis 的 zset 数据结构，实现一种时间戳有序集合。

集合的每个元素， member 和 score 都为时间戳(纳秒级别)。

漏桶大小为 count。

如果在（当前时间戳 - time）的时间戳内元素的个数超过 count 则说明漏桶有“溢出”。

###### 过期元素清除
和上面 list 数据结构基本类似，不同的是每次清理是清理 score 小于当前时间戳 - time的时间戳。

实现效果：*(本版本代码未完全实现)*

![image](https://raw.githubusercontent.com/heiyeluren/docs/master/imgs/koala_code_01.png)


### 问题模拟并解决
让我们来模拟上面count出现的问题并利用leak解决。

main.go

![image](https://raw.githubusercontent.com/heiyeluren/docs/master/imgs/koala_code_02.png)

运行结果如下图所示，可以看出count类型在第5秒6秒的时候通过了5次，而leak时刻保持5秒内最多访问3次。

![image](https://raw.githubusercontent.com/heiyeluren/docs/master/imgs/koala_code_03.png)

done.
