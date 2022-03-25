/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Dict list parser
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package koala

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strconv"
	"strings"
)

// RetValue retValue数据类型
type RetValue struct {
	RetType   int32
	RetCode   int32
	ErrNo     int32
	ErrMsg    string
	StrReason string
	NeedVcode int32
	VcodeLen  int32
	VcodeType int32
	Other     string
	Version   int32
}

// Policy .
// 策略结构，包含：dicts词表、rule规则、retValue 返回值三种数据;
// 其中 rule 是主体，dicts、和 retValue 会被 rule引用到
type Policy struct {
	dictsTable    map[string]map[string]string
	ruleTable     []Rule
	retValueTable map[int]RetValue
}

/**
 * 各种分隔符，trim的时候要除掉他们
 */
const emptyRunes = " \r\t\v"

var (
	// GlobalPolicy .全局策略配置
	GlobalPolicy *Policy
	// TempPolicy .临时策略配置（用于策略的动态更新）
	TempPolicy *Policy
)

// NewPolicy Policy构造函数，完成各个元素的空间初始化
func NewPolicy() *Policy {
	return &Policy{
		dictsTable:    make(map[string]map[string]string),
		ruleTable:     make([]Rule, 0, 50),
		retValueTable: make(map[int]RetValue),
	}
}

// PolicyInterpreter Policy解释器，用于从文件解析配置，记录到 Policy 结构中
func PolicyInterpreter(extStream string) error {
	// temp策略缓冲区初始化
	TempPolicy = NewPolicy()

	var rawStream []byte
	var err error
	if extStream != "" {
		rawStream = []byte(extStream)
	} else {
		rawStream, err = ioutil.ReadFile(Config.Get("rule_file"))
		if err != nil {
			return errors.New("cannot load rule file")
		}
	}

	DynamicUpdateFiles = append(DynamicUpdateFiles, Config.Get("rule_file"))
	lines := strings.Split(string(rawStream), "\n")

	// 配置文件分成三部分 词表、规则和返回结果，起始字符串分别是 [dicts] [rules] [result]
	// 寻找这三部分的起始行
	dictsPos, rulesPos, resultsPos := 0, 0, 0
	for index, line := range lines {
		line = strings.Trim(line, emptyRunes)
		if line == "" || line[0] == '#' {
			continue
		}
		if strings.EqualFold(strings.Trim(line, emptyRunes), "[dicts]") {
			dictsPos = index
		}
		if strings.EqualFold(strings.Trim(line, emptyRunes), "[rules]") {
			rulesPos = index
		}
		if strings.EqualFold(strings.Trim(line, emptyRunes), "[result]") {
			resultsPos = index
		}
	}

	// 解析词表配置
	for index := dictsPos + 1; index < rulesPos; index++ {
		line := strings.Trim(lines[index], emptyRunes)
		if line == "" || line[0] == '#' {
			continue
		}
		if err = dictsBuilder(line); err != nil {
			return errors.New(err.Error() + "  ;AT-LINE-" + strconv.Itoa(index) + "; " + line)
		}
	}

	// 解析规则配置
	for index := rulesPos + 1; index < resultsPos; index++ {
		line := strings.Trim(lines[index], emptyRunes)
		if line == "" || line[0] == '#' {
			continue
		}
		if err = rulesBuilder(line); err != nil {
			return errors.New(err.Error() + "  ;AT-LINE-" + strconv.Itoa(index) + "; " + line)
		}
		// println(line)
	}

	// 解析返回结果配置
	for index := resultsPos + 1; index < len(lines); index++ {
		line := strings.Trim(lines[index], emptyRunes)
		if line == "" || line[0] == '#' {
			continue
		}
		if err = resultsBuilder(line); err != nil {
			return errors.New(err.Error() + "  ;AT-LINE-" + strconv.Itoa(index) + "; " + line)
		}
	}

	// 校验规则有效性
	if err = ruleValidityCheck(); err != nil {
		return err
	}

	// 覆盖全局策略配置
	GlobalPolicy = TempPolicy

	return nil
}

/**
 * rule构造器，对单条 rule 进行解析 然后存入 TempPolicy
 */
func rulesBuilder(rule string) error {
	// rule : [direct] [qid @ global_qid_whitelist] [time=1; count=0;] [result=1; return=101]
	parts := strings.SplitN(rule, ":", 2)
	if len(parts) == 2 && strings.EqualFold(strings.Trim(parts[0], emptyRunes), "rule") {
		//
		var singleRule Rule
		if err := singleRule.Constructor(parts[1]); err != nil {
			return err
		}
		TempPolicy.ruleTable = append(TempPolicy.ruleTable, singleRule)
	} else {
		return errors.New("rule syntax error: struct error")
	}
	return nil
}

/**
 * dicts构造器，对单条 dict 进行解析 然后存入 TempPolicy
 */
func dictsBuilder(dict string) error {
	// 配置格式 名称 : 配置文件名
	// global_qid_whitelist : etc/global_qid_whitelist.dat

	parts := strings.SplitN(dict, ":", 2)
	if len(parts) != 2 {
		return errors.New("dict syntax error: struct error")
	}
	oneDict := make(map[string]string, 10)

	// 读取配置文件
	fileName := strings.Trim(parts[1], emptyRunes)
	DynamicUpdateFiles = append(DynamicUpdateFiles, fileName)
	rawStream, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.New("cannot load dict file")
	}
	lines := strings.Split(string(rawStream), "\n")
	for _, v := range lines {
		item := strings.Trim(v, emptyRunes)
		oneDict[item] = item
	}
	dictName := strings.Trim(parts[0], emptyRunes)
	TempPolicy.dictsTable[dictName] = oneDict
	return nil
}

/**
 * retValue 构造器，对单条 result 进行解析
 */
func resultsBuilder(result string) error {
	// 1 : { "Ret_type":1, "Ret_code" : 0, "Err_no":0, "Err_msg":"", "Str_reason":"Allow", "Need_vcode":0, "Vcode_len":0, "Vcode_type":0, "Other":"", "Version":0 }
	parts := strings.SplitN(result, ":", 2)
	var retType = -1
	var err error
	if retType, err = strconv.Atoi(strings.Trim(parts[0], emptyRunes)); err != nil {
		return err
	}

	var ret RetValue
	inString := strings.Trim(parts[1], emptyRunes)
	if err = json.Unmarshal([]byte(inString), &ret); err != nil {
		return err
	}
	// fmt.Printf("%+v \n", ret)
	TempPolicy.retValueTable[retType] = ret
	return nil
}

// ruleValidityCheck .
// 功能：rule合法性检查（事后检查）
// 对象：TempPolicy
// 作用：此检查发生在，解析过程的末尾，对已经读入内存的配置，检查其合法性、逻辑正确性
//       a、return值 唯一性检查
//       b、base、count、time、result、return完整性检查
//       c、base、count、time、result、return的范围检查，如 0 值、负值等
func ruleValidityCheck() error {
	var returnMap = make(map[int32]string)

	for _, singleRule := range TempPolicy.ruleTable {
		switch singleRule.method {
		case "direct":
			break
		case "count":
			if singleRule.count <= 0 || singleRule.time <= 0 {
				return errors.New("rule semantic error: rule argument out of range")
			}
		case "base":
			if singleRule.base <= 0 || singleRule.count <= 0 || singleRule.time <= 0 {
				return errors.New("rule semantic error: rule argument out of range")
			}
		default:
		}

		if singleRule.result <= 0 || singleRule.returnCode <= 0 {
			return errors.New("rule semantic error: result invalid")
		}

		if _, OK := TempPolicy.retValueTable[int(singleRule.result)]; !OK {
			return errors.New("rule semantic error: result type no found")
		}

		if _, OK := returnMap[singleRule.returnCode]; OK {
			return errors.New("rule semantic error: rules with same return code")
		}
		returnMap[singleRule.returnCode] = "hi"
	}
	return nil
}
