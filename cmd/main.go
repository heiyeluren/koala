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
	"github.com/heiyeluren/koala/koala"
)

/**
 * koala服务进程的main函数
 */
func main() {
	// Start koala
	koala.Run()

	// 启动 rule统计协程
	go koala.CounterAgent()

	// 启动 规则更新协程，定期检查 policy 更新
	go koala.PolicyLoader()

	// 启动 http监听协程
	go koala.FrontListen()

	// hold 住 main协程
	select {}
}
