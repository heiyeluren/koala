/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Dict list parser & loader
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/heiyeluren/koala/utility/logger"
)

// NewPolicyMD5 PolicyMd5 初始化函数
func NewPolicyMD5() string {
	m, err := PolicyMD5Str()
	if err != nil {
		// error log
	}
	return m
}

// PolicyLoader .
// policy实时更新函数
// 说明：用于实时更新rule配置，解析过程调用PolicyInterpreter()处理
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

		m, err = PolicyMD5Str()
		if err != nil {
			logHandle.Warning("[errmsg=" + err.Error() + " md5=" + PolicyMd5 + "]")
		}
		//println(Policy_md5)

		if m != PolicyMd5 {
			DynamicUpdateFiles = []string{}
			err = PolicyInterpreter("")
			if err != nil {
				logHandle.Warning("[errmsg=" + err.Error() + "]")
			} else {
				PolicyMd5 = m
				logHandle.Trace("[msg=policy reload! new-md5=" + PolicyMd5 + "]")
			}
		}
	}
}

// PolicyMD5Str 计算 DynamicUpdateFiles 所包含文件的 md5 值
func PolicyMD5Str() (string, error) {
	contentStream := ""
	for _, file := range DynamicUpdateFiles {
		rawStream, err := ioutil.ReadFile(file)
		if err != nil {
			return "", errors.New("cannot load policy file")
		}
		contentStream += string(rawStream)
	}

	filesMd5 := md5.New()
	io.WriteString(filesMd5, contentStream)

	stringMd5 := fmt.Sprintf("%x", filesMd5.Sum(nil))
	return stringMd5, nil
}
