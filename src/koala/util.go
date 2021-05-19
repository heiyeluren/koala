/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Utils functions
 *
 * @author: heiyeluren 
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package main

import (
    "errors"
    "io/ioutil"
    "os"
    "strconv"
    "strings"
)

/**
 * 进程 pid 记录函数
 */
func SavePid(pidFile string) {
    pid := os.Getpid()
    pidString := strconv.Itoa(pid)
    ioutil.WriteFile(pidFile, []byte(pidString), 0777)
}

/**
 * 判断字符串是否是 IP 地址
 */
func IsIPAddress(s string) bool {
    // 符合x.x.x.x模式的字符串，即认为是ip地址
    //println(s)
    parts := strings.Split(s, ".")
    if len(parts) == 4 {
        return true
    }
    return false
}

/**
 * 将代表ip地址、纯数字的string值 都统一转换成int64值
 */
func ToInteger64(s string) (int64, error) {
    var err error = nil
    var result int64 = 0
    if IsIPAddress(s) {
        parts := strings.Split(s, ".")
        for i, v := range parts {
            var intVal int
            if intVal, err = strconv.Atoi(v); err != nil {
                return result, err
            }
            if intVal > 255 || intVal < 0 {
                return result, errors.New("rule syntax error: scope error,out of ip range 0-255!")
            }
            result += int64(intVal) << uint((3-i)*8)
        }
    } else {
        var int64Val int64
        if int64Val, err = strconv.ParseInt(s, 10, 64); err != nil {
            return 0, err
        }
        result = int64Val
    }
    return result, nil
}
