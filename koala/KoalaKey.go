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

// KoalaKey interface ；用于支持多种 key 类型
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

// GroupKey 集合 key 类型；满足 KoalaKey interface
type GroupKey struct {
	set     map[string]string
	inverse bool // 取反标记；@,!@
	combine bool // 合并标记；{~}
}

/**
 * dump()
 */
func (g *GroupKey) dump() string {
	ret := ""
	if g.inverse {
		ret += "!"
	}
	if g.combine {
		ret += "~"
	}
	for v := range g.set {
		ret += v + ","
	}
	return ret
}

/**
 * build()
 */
func (g *GroupKey) build(sp, k, v string) error {
	// 词表识别
	g.set = make(map[string]string, 10)
	g.combine = false
	g.inverse = strings.HasSuffix(k, "!")
	var isPresent bool
	v = strings.Trim(v, emptyRunes)
	if sp == "@" {
		g.set, isPresent = TempPolicy.dictsTable[v]
		if !isPresent {
			return errors.New("rule build error: Dict not present")
		}
		return nil
	}
	if sp == "=" {
		g.combine = strings.HasSuffix(v, "{~}")
		v = strings.Trim(v, "{~}")
		elements := strings.Split(v, ",")
		for _, e := range elements {
			item := strings.Trim(e, emptyRunes)
			g.set[item] = item
		}
	}
	return nil
}

/**
 * matches()
 */
func (g *GroupKey) matches(s string) bool {
	s = strings.Trim(s, emptyRunes)
	if _, OK := g.set[s]; OK != g.inverse {
		return true
	}
	return false
}

// RangeKey 范围key；一个范围key可以包含多个范围区间
type RangeKey struct {
	scopes  []*Scope
	inverse bool // 取反标记 !
}

/**
 * dump
 */
func (k *RangeKey) dump() string {
	ret := ""
	if k.inverse {
		ret += "!"
	}
	for _, scop := range k.scopes {
		ret += scop.dump() + "&"
	}
	return ret
}

/**
 * build
 */
func (k *RangeKey) build(sp, ki, v string) error {
	// 范围识别
	var err error
	var int64Val int64
	k.inverse = strings.HasSuffix(ki, "!")
	v = strings.Trim(v, emptyRunes)
	if sp == "<" {
		oneScope := new(Scope)
		oneScope.op = sp
		if int64Val, err = strconv.ParseInt(v, 10, 64); err != nil {
			return errors.New("rule syntax error: < error,not integer")
		}
		oneScope.start = int64Val
		k.scopes = []*Scope{oneScope}
		return nil
	}
	if sp == ">" {
		oneScope := new(Scope)
		oneScope.op = sp
		if int64Val, err = strconv.ParseInt(v, 10, 64); err != nil {
			return errors.New("rule syntax error: > error,not integer")
		}
		oneScope.end = int64Val
		k.scopes = []*Scope{oneScope}
		return nil
	}
	// sp 是 =
	if v == "+" {
		oneScope := new(Scope)
		oneScope.op = v
		k.scopes = []*Scope{oneScope}
		return nil
	}
	k.scopes = []*Scope{}
	parts := strings.Split(v, ",")
	for _, sc := range parts {
		oneScope := new(Scope)
		if err = oneScope.build(sc); err != nil {
			return err
		}
		k.scopes = append(k.scopes, oneScope)
	}
	return nil
}

/**
 * matches
 */
func (k *RangeKey) matches(s string) bool {
	// + 号，任意值逻辑，直接matche
	for _, sco := range k.scopes {
		if sco.op == "+" {
			return true
		}
	}
	int64val, err := ToInteger64(s)
	if err != nil {
		return false
	}
	isIn := false
	for _, sco := range k.scopes {
		if sco.matches(int64val) {
			isIn = true
			break
		}
	}

	return isIn != k.inverse
}

// Scope 范围 scope 类型，标识一个数值区间，如：>100, 1-9
type Scope struct {
	op    string // -,+,>,<
	start int64
	end   int64
}

/**
 * dump
 */
func (s *Scope) dump() string {
	a := strconv.FormatInt(s.start, 10)
	b := strconv.FormatInt(s.end, 10)
	return s.op + "^" + a + "^" + b
}

/**
 * build
 */
func (s *Scope) build(sc string) error {
	var err error
	s.op = "-"
	parts := strings.Split(sc, "-")
	if len(parts) == 2 {
		s.start, err = ToInteger64(strings.Trim(parts[0], emptyRunes))
		if err != nil {
			return err
		}
		s.end, err = ToInteger64(strings.Trim(parts[1], emptyRunes))
		if err != nil {
			return err
		}
		return nil
	}
	if strings.ContainsAny(sc, "*") && IsIPAddress(sc) {
		rep := strings.NewReplacer("*", "0")
		addStart := strings.Trim(rep.Replace(sc), emptyRunes)
		s.start, err = ToInteger64(addStart)
		if err != nil {
			return err
		}
		rep = strings.NewReplacer("*", "255")
		addEnd := strings.Trim(rep.Replace(sc), emptyRunes)
		s.end, err = ToInteger64(addEnd)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("rule syntax error: scope error")
}

/**
 * matches
 */
func (s *Scope) matches(v int64) bool {
	switch s.op {
	case "+":
		return true
	case "<":
		if v < s.start {
			return true
		}
	case ">":
		if v > s.end {
			return true
		}
	case "-":
		if v >= s.start && v <= s.end {
			return true
		}
	default:
	}
	return false
}
