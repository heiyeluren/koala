package koala

import (
	"runtime"

	"github.com/gomodule/redigo/redis"
	"github.com/heiyeluren/koala/utility"
)

// @Project: koala
// @Author: houseme
// @Description:
// @File: koala
// @Version: 1.0.0
// @Date: 2021/7/13 00:16
// @Package koala
var (
	// Config koala基本配置
	Config *utility.Config
	// RedisPool 全局redis连接池
	RedisPool *redis.Pool

	// PolicyMd5 .
	// 全局 md5 值
	// 说明：定期检查 rule配置的变化，与此 md5 比较；实现动态更新规则
	PolicyMd5 string

	// DynamicUpdateFiles .
	// 需要检查更新的文件列表
	// 范围：rule 文件 + dicts 文件
	// 赋值：每次reload规则时，将其中的 dicts 文件记录于此
	DynamicUpdateFiles []string
)

func init() {
	// 设置koala进程并发线程数
	runtime.GOMAXPROCS(runtime.NumCPU())
	// 初始化配置
	Config = NewConfig()
	// 初始化连接池
	InitRedisPool()
}

// Run .
func Run() {
	// 保存进程的 pid 到文件中，供 stop、restart 脚本引用
	SavePid(Config.Get("pid_file"))

	// 初始化，并启动 logger 协程
	go utility.LogRun(Config.GetAll())

	// 首次加载规则
	if err := PolicyInterpreter(""); err != nil {
		panic(err.Error())
	}
	PolicyMd5 = NewPolicyMD5()
}
