/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Main router
 *
 * koala频率控制服务 (规则引擎)
 * 用途：是为了解决用户提交等相关频率控制的一个通用服务，主要是为了替换传统写死代码的频率控制模块以达到 高性能、灵活配置的要求。
 * 方案：支持高度灵活的规则配置；并实现了规则配置的动态加载； 后端cache采用带连接池的redis。
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package main

import (
	"runtime"

	"github.com/gomodule/redigo/redis"
	"github.com/heiyeluren/koala/utility/configs"
	"github.com/heiyeluren/koala/utility/logger"
)

var (
	// Config koala基本配置
	Config *configs.Config
	// RedisPool 全局redis连接池
	RedisPool *redis.Pool

	// PolicyMd5 .
	// 全局 md5 值
	// 说明：定期检查 rule配置的变化，与此 md5 比较；实现动态更新规则
	PolicyMd5 string

	//DynamicUpdateFiles
	// 需要检查更新的文件列表
	// 范围：rule 文件 + dicts 文件
	// 赋值：每次reload规则时，将其中的 dicts 文件记录于此
	DynamicUpdateFiles []string = []string{}
)

func init() {
	// 设置koala进程并发线程数
	runtime.GOMAXPROCS(runtime.NumCPU())
	// 初始化配置
	Config = newConfig()
	// 初始化连接池
	initRedisPool()
}

/**
 * koala服务进程的main函数
 */
func main() {
	// 保存进程的 pid 到文件中，供 stop、restart 脚本引用
	SavePid(Config.Get("pid_file"))

	// 初始化，并启动 logger 协程
	go logger.Log_Run(Config.GetAll())

	// 首次加载规则
	var err error = PolicyInterpreter("")
	if err != nil {
		panic(err.Error())
	}
	PolicyMd5 = NewPolicyMD5()

	// 启动 rule统计协程
	go CounterAgent()

	// 启动 规则更新协程，定期检查 policy 更新
	go PolicyLoader()

	// 启动 http监听协程
	go FrontListen()

	// hold 住 main协程
	select {}
}
