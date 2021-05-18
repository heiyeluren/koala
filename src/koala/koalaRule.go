package main

import (
    "errors"
    "github.com/garyburd/redigo/redis"
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

/************************************************************
                KoalaRule 构建，build，相关方法
************************************************************/

/**
 * KoalaRule 的构造器
 */
func (this *KoalaRule) Constructor(r string) error {
    // [direct] [qid @ global_qid_whitelist] [time=1; count=0;] [result=1; return=101]
    // [count] [act=ask;qid=+;] [time=2; count=1;] [result=2; return=201]
    // [base] [act=ask;ip=+;] [base=50; time=10; count=1;] [result=2; return=203]
    sections := strings.Split(r, "] [")
    if len(sections) != 4 {
        return errors.New("rule syntax error: section error!")
    }
    for i, _ := range sections {
        sections[i] = strings.Trim(sections[i], emptyRunes+"[]")
    }
    this.methord = sections[0]
    if this.methord != "count" && this.methord != "base" && this.methord != "direct" && this.methord != "leak" {
        return errors.New("rule syntax error: methord error!")
    }
    this.keys = make(map[string]KoalaKey, 10)
    if err := this.getKeys(sections[1]); err != nil {
        return err
    }
    // 解析剩余的 count、returnCode 等 参数
    if err := this.getCountAndRet(sections[2], sections[3]); err != nil {
        return err
    }
    return nil
}

/**
 * 解析 KoalaRule 的 count、returnCode 等 参数
 */
func (this *KoalaRule) getCountAndRet(val, ret string) error {
    val = strings.Trim(val, emptyRunes+";")
    ret = strings.Trim(ret, emptyRunes+";")
    vals := strings.Split(val, ";")
    for _, s := range vals {
        s = strings.Trim(s, emptyRunes)
        parts := strings.SplitN(s, "=", 2)
        if len(parts) != 2 {
            return errors.New("rule syntax error: value error!")
        }
        valueName := strings.Trim(parts[0], emptyRunes)
        valueData, err := strconv.Atoi(strings.Trim(parts[1], emptyRunes))
        if err != nil {
            return errors.New("rule syntax error: value error!")
        }
        switch valueName {
        case "base":
            this.base = int32(valueData)
        case "time":
            this.time = int32(valueData)
        case "count":
            this.count = int32(valueData)
        case "erase1":
            this.erase1 = int32(valueData)
        case "erase2":
            this.erase2 = int32(valueData)
        default:
            return errors.New("rule syntax error: value error!")
        }
    }
    rets := strings.Split(ret, ";")
    for _, s := range rets {
        s = strings.Trim(s, emptyRunes)
        parts := strings.SplitN(s, "=", 2)
        if len(parts) != 2 {
            return errors.New("rule syntax error: value error!")
        }
        valueName := strings.Trim(parts[0], emptyRunes)
        valueData, err := strconv.Atoi(strings.Trim(parts[1], emptyRunes))
        if err != nil {
            return errors.New("rule syntax error: value error!")
        }
        switch valueName {
        case "result":
            this.result = int32(valueData)
        case "return":
            this.returnCode = int32(valueData)
        default:
            return errors.New("rule syntax error: value error!")
        }
    }
    return nil
}

/**
 * 解析 KoalaRule 的 keys 参数
 */
func (this *KoalaRule) getKeys(k string) error {
    // act=ask;ip=+;
    k = strings.Trim(k, emptyRunes+";")
    allKey := strings.Split(k, ";")
    for i, _ := range allKey {
        allKey[i] = strings.Trim(allKey[i], emptyRunes)

        // 抽取 词表 @语句 集合key
        // qid @ global_qid_whitelist
        parts := strings.SplitN(allKey[i], "@", 2)
        if len(parts) == 2 {
            key_value := new(GroupKey)
            if err := key_value.build("@", parts[0], parts[1]); err != nil {
                return err
            }
            key_name := strings.Trim(parts[0], emptyRunes+"!")
            this.keys[key_name] = key_value
            continue
        }

        // 抽取 小于 < 语句 范围 key
        parts = strings.SplitN(allKey[i], "<", 2)
        if len(parts) == 2 {
            key_value := new(RangeKey)
            if err := key_value.build("<", parts[0], parts[1]); err != nil {
                return err
            }
            key_name := strings.Trim(parts[0], emptyRunes+"!")
            this.keys[key_name] = key_value
            continue
        }

        // 抽取 小于 > 语句 范围key
        parts = strings.SplitN(allKey[i], ">", 2)
        if len(parts) == 2 {
            key_value := new(RangeKey)
            if err := key_value.build(">", parts[0], parts[1]); err != nil {
                return err
            }
            key_name := strings.Trim(parts[0], emptyRunes+"!")
            this.keys[key_name] = key_value
            continue
        }

        // 处理其他的 以 = 分割的语句，否则报错
        parts = strings.SplitN(allKey[i], "=", 2)
        if len(parts) != 2 {
            return errors.New("rule syntax error: keys error,miss sp =!")
        }
        if strings.ContainsAny(parts[1], "+-*") {
            key_value := new(RangeKey) // 范围
            if err := key_value.build("=", parts[0], parts[1]); err != nil {
                return err
            }
            key_name := strings.Trim(parts[0], emptyRunes+"!")
            this.keys[key_name] = key_value
        } else {
            key_value := new(GroupKey) // 集合
            if err := key_value.build("=", parts[0], parts[1]); err != nil {
                return err
            }
            key_name := strings.Trim(parts[0], emptyRunes+"!")
            this.keys[key_name] = key_value
        }

    }
    return nil
}

/************************************************************
                KoalaRule 使用过程，matche，相关方法
************************************************************/

/**
 * 缓存 key 拼装函数
 */
func (this *KoalaRule) getCacheKey(gets map[string]string) string {
    // cacheKey，先加上 r101 形式的前缀，代表所属规则，101等同于规则returnCode
    cacheKey := "r" + strconv.Itoa(int(this.returnCode))

    kForSort := make([]string, len(this.keys))
    i := 0
    for keyName, _ := range this.keys {
        kForSort[i] = keyName
        i++
    }
    // 因map遍历时的顺序随机不定，但cache key必须确保一致；
    // 故先对map的key字典序排序，后面按此顺序引用map值
    sort.Strings(kForSort)

    for _, keyName := range kForSort {
        keyValue := this.keys[keyName]
        switch keyValue.(type) {
        case *GroupKey:
            // combine的groupkey，不拼入，达到combine效果(联合计数)
            if !keyValue.(*GroupKey).combine {
                cacheKey = cacheKey + "|" + gets[keyName]
            }

        default:
            cacheKey = cacheKey + "|" + gets[keyName]
        }
    }
    cacheKey = strings.Trim(cacheKey, "|")
    return cacheKey
}

/**
 * 查询：查询规则当前的缓存值
 */
func (this *KoalaRule) getCacheValue(cacheKey string) (int, error) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var err error
    var cacheValue int
    cacheValue, err = redis.Int(redisConn.Do("GET", cacheKey))
    if err == redis.ErrNil {
        // 此 key 不存在，直接返回 0
        return 0, nil
    }
    if err != nil {
        return 0, err
    }
    //println(cacheKey, " --> value:", cacheValue)
    return cacheValue, nil
}

/**
 * 浏览；count规则缓存查询、比较
 */
func (this *KoalaRule) countBrowse(cacheKey string) (bool, error) {
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
    if this.count == 0 || this.count > int32(cacheValue) {
        return false, nil
    }
    return true, nil
}

/**
 * 更新；count规则缓存更新
 */
func (this *KoalaRule) countUpdate(cacheKey string) error {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var exists int
    var err error
    if exists, err = redis.Int(redisConn.Do("EXISTS", cacheKey)); err != nil {
        return err
    }
    // set new cache
    if exists == 0 {
        var expiretime int32
        // 86400 按照 自然天计算 过期时间
        if this.time == 86400 {
            y, m, d := time.Now().Date()
            loc, _ := time.LoadLocation("Asia/Shanghai")
            dayEnd := time.Date(y, m, d, 23, 59, 59, 0, loc).Unix()
            expiretime = int32(dayEnd - time.Now().Unix())
        } else {
            expiretime = this.time
        }

        if _, err := redis.String(redisConn.Do("SETEX", cacheKey, expiretime, 1)); err != nil {
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

/**
 * 浏览；base方法缓存查询、比较
 */
func (this *KoalaRule) baseBrowse(cacheKey string) (bool, error) {
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
    if this.base == 0 || this.base > int32(cacheValue) {
        return false, nil
    }
    var cacheKey_time string = cacheKey + BASE_KEY_SUFFIX

    cacheValue, err = redis.Int(redisConn.Do("GET", cacheKey_time))
    if err == redis.ErrNil {
        // 此 key 不存在，直接返回通过
        return false, nil
    }
    if err != nil {
        return false, err
    }

    //println(cacheKey_time, " --> value:", cacheValue)
    if this.count == 0 || this.count > int32(cacheValue) {
        return false, nil
    }
    return true, nil
}

/**
 * 更新；base方法缓存更新
 */
func (this *KoalaRule) baseUpdate(cacheKey string) error {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var exists int
    var err error
    if exists, err = redis.Int(redisConn.Do("EXISTS", cacheKey)); err != nil {
        return err
    }
    if exists == 0 {
        y, m, d := time.Now().Date()
        dayEnd := time.Date(y, m, d, 23, 59, 59, 0, time.UTC).Unix()
        expiretime := int32(dayEnd - time.Now().Unix())
        if _, err := redis.String(redisConn.Do("SETEX", cacheKey, expiretime, 1)); err != nil {
            return err
        }

        return nil
    }

    // update
    var cacheValue int
    if cacheValue, err = redis.Int(redisConn.Do("INCR", cacheKey)); err != nil {
        return err
    }
    if this.base == 0 || this.base > int32(cacheValue) {
        return nil
    }

    var cacheKey_time string = cacheKey + BASE_KEY_SUFFIX
    if exists, err = redis.Int(redisConn.Do("EXISTS", cacheKey_time)); err != nil {
        return err
    }
    if exists == 0 {
        if _, err := redis.String(redisConn.Do("SETEX", cacheKey_time, this.time, 1)); err != nil {
            return err
        }
        return nil
    }
    if _, err := redis.Int(redisConn.Do("INCR", cacheKey_time)); err != nil {
        return err
    }
    return nil
}

/**
 * leak模式--查询
 *
 */
func (this *KoalaRule) leakBrowse(cacheKey string) (bool, error) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    var err error
    var listLen int
    var edgeElement int64
    if listLen, err = redis.Int(redisConn.Do("LLEN", cacheKey)); err != nil {
        return false, err
    }
    if listLen == 0 || listLen <= int(this.count) {
        return false, nil
    }

    defer func() {
        go this.leakClear(cacheKey, listLen)
    }()

    now := time.Now().Unix()
    if edgeElement, err = redis.Int64(redisConn.Do("LINDEX", cacheKey, this.count)); err != nil {
        return false, err
    }
    if int32(now-edgeElement) <= this.time {
        return true, nil
    }
    return false, nil
}

/**
 * leak模式--清理
 * 清理队尾过期多余元素
 */
func (this *KoalaRule) leakClear(cacheKey string, listLen int) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    for listLen > int(this.count+1) {
        if _, err := redis.Int64(redisConn.Do("RPOP", cacheKey)); err != nil {
            return
        }
        listLen--
    }
}

/**
 * leak模式--更新
 */
func (this *KoalaRule) leakUpdate(cacheKey string) error {
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
func (this *KoalaRule) leakFeedback(cacheKey string, feedback int) error {
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

/**
 * 多重浏览；direct规则直接判定
 */
func (this *KoalaRule) multiDirectBrowse(cacheKeys []interface{}) (map[string]bool, error) {
    return nil, nil
}

/**
 * 多重浏览；count规则缓存查询、比较
 */
func (this *KoalaRule) multiCountBrowse(cacheKeys []interface{}) (map[string]bool, error) {
    redisConn := RedisPool.Get()
    defer redisConn.Close()

    multiResult := make(map[string]bool, len(cacheKeys))
    cacheVals := make([]int, len(cacheKeys))
    intf := []interface{}{}
    for i, _ := range cacheVals {
        intf = append(intf, &cacheVals[i])
    }
    reply, err := redis.Values(redisConn.Do("MGET", cacheKeys...))
    if err != nil {
        return nil, err
    }
    if _, err := redis.Scan(reply, intf...); err != nil {
        return nil, err
    }

    for i, v := range cacheVals {
        key := cacheKeys[i].(string)
        if this.count == 0 || v == 0 || this.count > int32(v) {
            multiResult[key] = false
            continue
        }
        multiResult[key] = true
    }
    return multiResult, nil
}

/**
 * 多重浏览；base方法缓存查询、比较
 */
func (this *KoalaRule) multiBaseBrowse(cacheKeys []interface{}) (map[string]bool, error) {
    return nil, nil
}
