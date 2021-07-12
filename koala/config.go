/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - koala server config parse
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package koala

import (
	"flag"

	"github.com/heiyeluren/koala/utility"
)

// NewConfig 启动前加载配置文件
func NewConfig() *utility.Config {
	var F string
	flag.StringVar(&F, "f", "", "config file")
	flag.Parse()
	if F == "" {
		panic("usage: ./koala -f etc/koala.conf")
	}
	config := utility.NewConfig()
	if err := config.Load(F); err != nil {
		panic(err.Error())
	}
	return config
}
