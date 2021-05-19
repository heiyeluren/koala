/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Test case code
 *
 * @author: heiyeluren 
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package main

// 测试使用 暂时删除
/*
import (
    "fmt"
    "github.com/garyburd/redigo/redis"
    "runtime"
    "strconv"
    "time"
    "utility/logger"
    "utility/network"
)

func (this *FrontServer) DoLogTest(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    logHandle.Debug("[clientip=192.168.0.1 errno=0 errmsg=ok  debuginfo]")
    logHandle.Trace("[clientip=192.168.0.1 errno=0 errmsg=ok  traceinfo]")
    logHandle.Warning("warning test")

    response.Puts(request.Gstr("name"))

    response.SetCode(200)
}

func (this *FrontServer) DoDumpPolicy(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    println(request.GetRemoteIP())

    var dumpString string = ""
    var localPolicy *Policy = GlobalPolicy
    for _, singleRule := range localPolicy.ruleTable {
        var singleDump string = ""
        singleDump += "methord_ " + singleRule.methord
        singleDump += " _keys_ " + dumpkeys(singleRule.keys)
        singleDump += " _base_ " + strconv.Itoa(int(singleRule.base))
        singleDump += " _time_ " + strconv.Itoa(int(singleRule.time))
        singleDump += " _count_ " + strconv.Itoa(int(singleRule.count))
        singleDump += " _result_ " + strconv.Itoa(int(singleRule.result))
        singleDump += " _returnCode_ " + strconv.Itoa(int(singleRule.returnCode)) + "<br />" + "<br />"
        dumpString += singleDump
    }
    response.Puts(dumpString)
    response.SetCode(200)
}

func dumpkeys(keys map[string]KoalaKey) string {
    ret := ""
    for k, v := range keys {
        ret += k + "=" + v.dump() + ";"
    }
    return ret
}

func (this *FrontServer) DoRedisCmd(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    ret, err := redis.Int(redisConn.Do("get", "chenp0626"))
    println(ret)
    if err == redis.ErrNil {
        response.Puts("redis error" + err.Error())
    }

    //response.Puts(FileAndLines() + "<br />")
    if ret == 0 {
        result, err := redis.String(redisConn.Do("setex", "chenp0626", 6500, 102400))
        if err != nil {
            response.Puts("redis error" + err.Error())
            response.SetCode(200)
            return
        }
        response.Puts(result)
    }
    response.Puts("OK!!")
    response.SetCode(200)
}

func (this *FrontServer) DoRedisMget(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    _, err := redis.String(redisConn.Do("set", "nokey", time.Now().Unix()))
    if err != nil {
        response.Puts("redis error" + err.Error())
        return
    }
    _, err = redis.String(redisConn.Do("set", "foo", time.Now().Unix()+5))
    if err != nil {
        response.Puts("redis error" + err.Error())
        return
    }

    cmd := []interface{}{"nokey", "foo"}
    vals := make([]int, 2)
    intf := []interface{}{}
    for i, _ := range vals {
        intf = append(intf, &vals[i])
    }

    reply, err := redis.Values(redisConn.Do("MGET", cmd...))
    if err != nil {
        response.Puts("redis Values error" + err.Error())
        return
    }
    if _, err := redis.Scan(reply, intf...); err != nil {
        response.Puts("redis Scan error" + err.Error())
        return
    }

    response.Puts("dump ::: " + fmt.Sprintf("%d", vals))
    response.SetCode(200)
}

func FileAndLines() (ret string) {
    fcName, file, line, ok := runtime.Caller(1)
    funcName := ""
    if ok {
        funcName = runtime.FuncForPC(fcName).Name()
    }
    return funcName + "~~~" + file + " ~~~" + strconv.Itoa(line)
}

*/
