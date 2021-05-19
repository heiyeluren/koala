/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine code
 *
 * @author: heiyeluren 
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package main

import (
    "errors"
    "strconv"
    "strings"
)

/**
 * KoalaKey interface ；用于支持多种 key 类型
 */
type KoalaKey interface {
    // 从配置字符串识别并生成 key 结构
    build(op, k, v string) error

    // 匹配 s 是否包含于 key 的范围
    matches(s string) bool

    // 仅调试用途
    dump() string
}

// KoalaKey 有两个子类型
// 集合 GroupKey
// 范围 RangeKey

/**
 * 集合 key 类型；满足 KoalaKey interface
 */
type GroupKey struct {
    set     map[string]string
    inverse bool // 取反标记；@,!@
    combine bool // 合并标记；{~}
}

/**
 * dump()
 */
func (this *GroupKey) dump() string {
    ret := ""
    if this.inverse {
        ret += "!"
    }
    if this.combine {
        ret += "~"
    }
    for v, _ := range this.set {
        ret += v + ","
    }
    return ret
}

/**
 * build()
 */
func (this *GroupKey) build(sp, k, v string) error {
    // 词表识别
    this.set = make(map[string]string, 10)
    this.combine = false
    this.inverse = strings.HasSuffix(k, "!")
    var isPresent bool
    v = strings.Trim(v, emptyRunes)
    if sp == "@" {
        this.set, isPresent = TempPolicy.dictsTable[v]
        if !isPresent {
            return errors.New("rule build error: Dict not present!")
        }
        return nil
    }
    if sp == "=" {
        this.combine = strings.HasSuffix(v, "{~}")
        v = strings.Trim(v, "{~}")
        elements := strings.Split(v, ",")
        for _, e := range elements {
            item := strings.Trim(e, emptyRunes)
            this.set[item] = item
        }
    }
    return nil
}

/**
 * matches()
 */
func (this *GroupKey) matches(s string) bool {
    s = strings.Trim(s, emptyRunes)
    if _, OK := this.set[s]; OK != this.inverse {
        return true
    }
    return false
}

/**
 * 范围key；一个范围key可以包含多个范围区间
 */
type RangeKey struct {
    scopes  []*Scope
    inverse bool // 取反标记 !
}

/**
 * dump
 */
func (this *RangeKey) dump() string {
    ret := ""
    if this.inverse {
        ret += "!"
    }
    for _, scop := range this.scopes {
        ret += scop.dump() + "&"
    }
    return ret
}

/**
 * build
 */
func (this *RangeKey) build(sp, k, v string) error {
    // 范围识别
    var err error = nil
    var int64Val int64
    this.inverse = strings.HasSuffix(k, "!")
    v = strings.Trim(v, emptyRunes)
    if sp == "<" {
        oneScope := new(Scope)
        oneScope.op = sp
        if int64Val, err = strconv.ParseInt(v, 10, 64); err != nil {
            return errors.New("rule syntax error: < error,not integer!")
        }
        oneScope.start = int64Val
        this.scopes = []*Scope{oneScope}
        return nil
    }
    if sp == ">" {
        oneScope := new(Scope)
        oneScope.op = sp
        if int64Val, err = strconv.ParseInt(v, 10, 64); err != nil {
            return errors.New("rule syntax error: > error,not integer!")
        }
        oneScope.end = int64Val
        this.scopes = []*Scope{oneScope}
        return nil
    }
    // sp 是 =
    if v == "+" {
        oneScope := new(Scope)
        oneScope.op = v
        this.scopes = []*Scope{oneScope}
        return nil
    }
    this.scopes = []*Scope{}
    parts := strings.Split(v, ",")
    for _, sc := range parts {
        oneScope := new(Scope)
        if err = oneScope.build(sc); err != nil {
            return err
        }
        this.scopes = append(this.scopes, oneScope)
    }
    return nil
}

/**
 * matches
 */
func (this *RangeKey) matches(s string) bool {
    // + 号，任意值逻辑，直接matche
    for _, sco := range this.scopes {
        if sco.op == "+" {
            return true
        }
    }
    int64val, err := ToInteger64(s)
    if err != nil {
        return false
    }
    isIn := false
    for _, sco := range this.scopes {
        if sco.matches(int64val) {
            isIn = true
            break
        }
    }

    return isIn != this.inverse
}

/**
 * 范围 scope 类型，标识一个数值区间，如：>100, 1-9
 */
type Scope struct {
    op    string // -,+,>,<
    start int64
    end   int64
}

/**
 * dump
 */
func (this *Scope) dump() string {
    a := strconv.FormatInt(this.start, 10)
    b := strconv.FormatInt(this.end, 10)
    return this.op + "^" + a + "^" + b
}

/**
 * build
 */
func (this *Scope) build(sc string) error {
    var err error = nil
    this.op = "-"
    parts := strings.Split(sc, "-")
    if len(parts) == 2 {
        this.start, err = ToInteger64(strings.Trim(parts[0], emptyRunes))
        if err != nil {
            return err
        }
        this.end, err = ToInteger64(strings.Trim(parts[1], emptyRunes))
        if err != nil {
            return err
        }
        return nil
    }
    if strings.ContainsAny(sc, "*") && IsIPAddress(sc) {
        rep := strings.NewReplacer("*", "0")
        add_start := strings.Trim(rep.Replace(sc), emptyRunes)
        this.start, err = ToInteger64(add_start)
        if err != nil {
            return err
        }
        rep = strings.NewReplacer("*", "255")
        add_end := strings.Trim(rep.Replace(sc), emptyRunes)
        this.end, err = ToInteger64(add_end)
        if err != nil {
            return err
        }
        return nil
    }
    return errors.New("rule syntax error: scope error!")
}

/**
 * matches
 */
func (this *Scope) matches(v int64) bool {
    switch this.op {
    case "+":
        return true
    case "<":
        if v < this.start {
            return true
        }
    case ">":
        if v > this.end {
            return true
        }
    case "-":
        if v >= this.start && v <= this.end {
            return true
        }
    default:
    }
    return false
}
