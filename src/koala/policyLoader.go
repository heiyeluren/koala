package main

import (
    "crypto/md5"
    "errors"
    "fmt"
    "io"
    "io/ioutil"
    "time"
    "utility/logger"
)

/**
 * Policy_md5 初始化函数
 */
func NewPolicyMD5() string {
    m, err := Policy_MD5()
    if err != nil {
        // error log
    }
    return m
}

/**
 * policy实时更新函数
 * 说明：用于实时更新rule配置，解析过程调用PolicyInterpreter()处理
 */
func PolicyLoader() {
    var err error
    var m string
    logHandle := logger.NewLogger("")

    for {
        d := Config.GetInt("policy_loader_frequency")
        if d == 0 {
            d = 300
        }
        time.Sleep(time.Duration(d) * time.Second)

        m, err = Policy_MD5()
        if err != nil {
            logHandle.Warning("[errmsg=" + err.Error() + " md5=" + Policy_md5 + "]")
        }
        //println(Policy_md5)

        if m != Policy_md5 {
            DynamicUpdateFiles = []string{}
            err = PolicyInterpreter("")
            if err != nil {
                logHandle.Warning("[errmsg=" + err.Error() + "]")
            } else {
                Policy_md5 = m
                logHandle.Trace("[msg=policy reload! new-md5=" + Policy_md5 + "]")
            }
        }
    }
}

/**
 * 计算 DynamicUpdateFiles 所包含文件的 md5 值
 */
func Policy_MD5() (string, error) {
    contentStream := ""
    for _, file := range DynamicUpdateFiles {
        rawStream, err := ioutil.ReadFile(file)
        if err != nil {
            return "", errors.New("cannot load policy file!")
        }
        contentStream += string(rawStream)
    }

    files_MD5 := md5.New()
    io.WriteString(files_MD5, contentStream)

    string_md5 := fmt.Sprintf("%x", files_MD5.Sum(nil))
    return string_md5, nil
}
