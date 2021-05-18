package main

import (
    "encoding/json"
    "net/url"
    "strconv"
    "strings"
    "utility/logger"
    "utility/network"
)

/**
 *   查询访问接口
 */
func (this *FrontServer) DoRuleBrowse(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    // 本地策略指针，可避免匹配过程中Global策略被替换 导致不一致
    var localPolicy *Policy = GlobalPolicy

    var singleRule KoalaRule
    var err error
    var retValue RetValue = localPolicy.retValueTable[0]
    // 匹配每一条rule规则
    for _, singleRule = range localPolicy.ruleTable {
        var satisfied bool = true
        // 遍历每个key，若不匹配或者参数未传，不命中
        for k, v := range singleRule.keys {
            s := request.Gstr(k)
            if s != "" && v.matches(s) {
                continue
            }
            satisfied = false
            break
        }
        if !satisfied {
            continue
        }

        // 对命中的key，查缓存值，与阀值比较，判断是否超出限制
        var isOut bool
        ruleCacheKey := singleRule.getCacheKey(request.Gets())
        //println(ruleCacheKey)
        switch singleRule.methord {
        case "direct":
            isOut = true
        case "count":
            if isOut, err = singleRule.countBrowse(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
            }
        case "base":
            if isOut, err = singleRule.baseBrowse(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
            }
        case "leak":
            if isOut, err = singleRule.leakBrowse(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
            }
        default:
        }

        // 统计，记录策略判定数据
        CounterClient(singleRule.returnCode, isOut)

        // 超出限制，按照rule的约定，给出处置策略
        if isOut {
            retValue = localPolicy.retValueTable[int(singleRule.result)]
            retValue.Ret_code = singleRule.returnCode
            break
        }
        retValue = localPolicy.retValueTable[1]
    }

    // _writeThrough“直接写缓存”开关，同时完成 Browse和 Update两步操作。
    if retValue.Ret_type >= 1 && request.Gstr("_writeThrough") == "yes" {
        go RuleUpdateLogic(request, logHandle)
    }

    // 返回json结果
    var retString []byte
    retString, err = json.Marshal(retValue)
    if err != nil {
        response.SetCode(500)
        return
    }
    response.Puts(string(retString))

    response.SetCode(200)
}

/**
 *    更新访问接口
 */
func (this *FrontServer) DoRuleUpdate(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    go RuleUpdateLogic(request, logHandle)

    response.Puts(`{"err_no":0, "err_msg":"OK"}`)
    response.SetCode(200)
}

/**
 *    查询缓存状态接口
 */
/*
func (this *FrontServer) DoRuleValue(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    var localPolicy *Policy = GlobalPolicy

    var singleRule KoalaRule
    var err error
    var cacheVal int = -1
    var req_rule int32 = int32(request.Gint("rule_no"))
    for _, singleRule = range localPolicy.ruleTable {
        if singleRule.returnCode != req_rule {
            continue
        }
        ruleCacheKey := singleRule.getCacheKey(request.Gets())
        if cacheVal, err = singleRule.getCacheValue(ruleCacheKey); err != nil {
            logHandle.Fatal("[errmsg=" + err.Error() + "]")
            response.Puts(`{"err_no":-1, "err_msg":"system error!"}`)
            response.SetCode(500)
            break
        }
    }
    if cacheVal == -1 {
        response.Puts(`{"err_no":-2, "err_msg":"unknown rule_no!"}`)
        response.SetCode(400)
    } else {
        response.Puts(`{"err_no":0, "err_msg":"", "cache_val":` + strconv.Itoa(cacheVal) + `}`)
        response.SetCode(200)
    }
}
*/

/**
 *   非中断查询接口（可命中、并返回多条策略）
 */
func (this *FrontServer) DoRuleBrowse_complete(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    // 本地策略指针，可避免匹配过程中Global策略被替换 导致不一致
    var localPolicy *Policy = GlobalPolicy

    var singleRule KoalaRule
    var err error
    // 用于返回多个结果，RetValue数组
    var retArray []RetValue
    var retValue RetValue = localPolicy.retValueTable[0]
    // 匹配每一条rule规则
    for _, singleRule = range localPolicy.ruleTable {
        var satisfied bool = true
        // 遍历每个key，若不匹配或者参数未传，跳过
        for k, v := range singleRule.keys {
            s := request.Gstr(k)
            if s != "" && v.matches(s) {
                continue
            }
            satisfied = false
            break
        }
        if !satisfied {
            continue
        }

        // 对匹配的key，查缓存值，与阀值比较，判断是否超出限制
        var isOut bool
        switch singleRule.methord {
        case "direct":
            isOut = true
        case "count":
            ruleCacheKey := singleRule.getCacheKey(request.Gets())
            if isOut, err = singleRule.countBrowse(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
            }
        case "base":
            ruleCacheKey := singleRule.getCacheKey(request.Gets())
            if isOut, err = singleRule.baseBrowse(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
            }
        default:
        }

        // 统计，记录策略判定数据
        CounterClient(singleRule.returnCode, isOut)

        // 命中，拼装结果
        if isOut {
            retValue = localPolicy.retValueTable[int(singleRule.result)]
            retValue.Ret_code = singleRule.returnCode
            retArray = append(retArray, retValue)
        }
    }

    // 如果没有命中任何策略，返回默认值
    if retValue.Ret_type == 0 {
        retArray = append(retArray, retValue)
    }

    // _writeThrough“直接写缓存”开关，同时完成 Browse和 Update两步操作。
    if retValue.Ret_type <= 1 && request.Gstr("_writeThrough") == "yes" {
        RuleUpdateLogic(request, logHandle)
    }

    // 返回json结果
    var retString []byte
    retString, err = json.Marshal(retArray)
    if err != nil {
        response.SetCode(500)
        return
    }
    response.Puts(string(retString))

    response.SetCode(200)
}

/**
 *    反馈-feedback-接口
 */
/*
func (this *FrontServer) DoRuleFeedback(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {

    var localPolicy *Policy = GlobalPolicy

    var singleRule KoalaRule
    var feedbackType string = request.Gstr("_feedbackType")
    // 匹配每一条rule规则
    for _, singleRule = range localPolicy.ruleTable {
        var satisfied bool = true
        // 遍历每个key，若不匹配或者参数未传，不命中
        for k, v := range singleRule.keys {
            s := request.Gstr(k)
            if s != "" && v.matches(s) {
                continue
            }
            satisfied = false
            break
        }
        if !satisfied {
            continue
        }

        var feedback int32
        switch feedbackType {
        case "erase1":
            feedback = singleRule.erase1
        case "erase2":
            feedback = singleRule.erase2
        default:
            feedback = singleRule.erase1
        }

        ruleCacheKey := singleRule.getCacheKey(request.Gets())
        go singleRule.leakFeedback(ruleCacheKey, int(feedback))
    }

    response.Puts(`{"err_no":0, "err_msg":"OK"}`)
    response.SetCode(200)
}
*/

/**
 *   更新操作执行函数
 */
func RuleUpdateLogic(request *network.HttpRequest, logHandle *logger.Logger) {
    // 本地策略指针，可避免匹配过程中Global策略被替换 导致不一致
    var localPolicy *Policy = GlobalPolicy

    // 匹配每一条rule规则
    var singleRule KoalaRule
    for _, singleRule = range localPolicy.ruleTable {
        var satisfied bool = true
        // 遍历每个key，若不匹配或者参数未传，不命中
        for k, v := range singleRule.keys {
            s := request.Gstr(k)
            if s != "" && v.matches(s) {
                continue
            }
            satisfied = false
            break
        }
        if !satisfied {
            continue
        }
        var ruleCacheKey string
        var err error

        ruleCacheKey = singleRule.getCacheKey(request.Gets())
        // 更新cache值
        switch singleRule.methord {
        case "count":
            if singleRule.count == 0 {
                continue
            }
            if err = singleRule.countUpdate(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
                continue
            }
        case "base":
            if err = singleRule.baseUpdate(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
                continue
            }
        case "leak":
            if err = singleRule.leakUpdate(ruleCacheKey); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
                continue
            }
        default:
        }
    }
}

/**
 * 监控连接redis是否成功
 */
func (this *FrontServer) DoMonitorAlive(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()
    _, err := redisConn.Do("PING")
    if err != nil {
        response.Puts(`{"errno": -1, "errmsg": "redis-error!"}`)
        response.SetCode(500)
        logHandle.Fatal("[errmsg=" + err.Error() + "]")
        return
    }
    response.Puts(`{"errno": 0, "errmsg": "OK!"}`)
    response.SetCode(200)
}

type Job struct {
    Id  string
    Arg string
}

type JobResult struct {
    Id     string
    Result RetValue
}

type JobBuffer struct {
    Id       string
    args     map[string]string
    key      string
    status   bool
    decision int
    retCode  int32
}

/**
 * 多重浏览访问接口
 */
func (this *FrontServer) DoMultiBrowse(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    argsJson := request.Gstr("argsJson")
    if argsJson == "" {
        response.SetCode(400)
        return
    }

    var err error
    var jobs []Job
    if err = json.Unmarshal([]byte(argsJson), &jobs); err != nil {
        logHandle.Fatal("[errmsg=" + err.Error() + "]")
        response.SetCode(400)
        return
    }

    var logMsg string = ""
    logMsg += "[ cip=" + request.GetRemoteIP()
    logMsg += " intf=" + request.PathInfo()

    var buffers []JobBuffer
    //var job Job
    for _, job := range jobs {
        var buf JobBuffer
        buf.Id = job.Id
        buf.args = parseJobArgs(job.Arg)
        buf.status = false
        buf.decision = 0
        buffers = append(buffers, buf)
        logMsg += " ID" + job.Id + "@" + job.Arg
    }
    logMsg += " ] ["

    var localPolicy *Policy = GlobalPolicy
    var singleRule KoalaRule
    for _, singleRule = range localPolicy.ruleTable {
        var cacheKeys []interface{}
        for i, buf := range buffers {
            buffers[i].key = ""
            var satisfied bool = true
            for k, v := range singleRule.keys {
                s, OK := buf.args[k]
                if OK && v.matches(s) {
                    continue
                }
                satisfied = false
                break
            }
            if satisfied == true && buf.status == false {
                buffers[i].key = singleRule.getCacheKey(buf.args)
                cacheKeys = append(cacheKeys, buffers[i].key)
            }
        }

        if len(cacheKeys) == 0 {
            continue
        }

        var multiResult map[string]bool
        switch singleRule.methord {
        case "direct":
            if multiResult, err = singleRule.multiDirectBrowse(cacheKeys); err != nil {
                // err log
            }
        case "count":
            if multiResult, err = singleRule.multiCountBrowse(cacheKeys); err != nil {
                logHandle.Fatal("[errmsg=" + err.Error() + "]")
            }
        case "base":
            if multiResult, err = singleRule.multiBaseBrowse(cacheKeys); err != nil {
                // err log
            }
        default:
        }

        // 统计，记录策略判定数据
        for _, decision := range multiResult {
            CounterClient(singleRule.returnCode, decision)
        }

        for i, buf := range buffers {
            isOut, OK := multiResult[buf.key]
            if OK && isOut {
                buffers[i].status = true
                buffers[i].decision = int(singleRule.result)
                buffers[i].retCode = singleRule.returnCode
            } else if OK && buf.status == false {
                buffers[i].decision = 1
            }
        }
    }

    var jobResults []JobResult
    for _, buf := range buffers {
        var singleResult JobResult
        singleResult.Id = buf.Id
        singleResult.Result = localPolicy.retValueTable[buf.decision]
        singleResult.Result.Ret_code = buf.retCode
        jobResults = append(jobResults, singleResult)
        logMsg += " ID" + buf.Id + "~Ret_code:" + strconv.Itoa(int(buf.retCode))
    }
    logMsg += " ]"
    logHandle.Notice(logMsg)

    // 返回json结果
    var retString []byte
    retString, err = json.Marshal(jobResults)
    if err != nil {
        response.SetCode(500)
        return
    }
    response.Puts(string(retString))

    response.SetCode(200)

}

/**
 *
 */
func parseJobArgs(rawArg string) map[string]string {
    retMap := make(map[string]string, 0)
    argString, err := url.QueryUnescape(rawArg)
    if err != nil {
        return nil
    }
    kvStrings := strings.Split(string(argString), "&")
    for _, kvString := range kvStrings {
        parts := strings.SplitN(kvString, "=", 2)
        if len(parts) != 2 {
            return nil
        }
        retMap[parts[0]] = parts[1]
    }
    return retMap
}
