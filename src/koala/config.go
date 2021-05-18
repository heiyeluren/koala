package main

import (
    "flag"
    "utility/configs"
)

// 启动前加载配置文件
func newConfig() *configs.Config {
    var F string
    flag.StringVar(&F, "f", "", "config file")
    flag.Parse()
    if F == "" {
        panic("usage: ./koala -f etc/koala.conf")
    }
    Config := configs.NewConfig()
    if err := Config.Load(F); err != nil {
        panic(err.Error())
    }
    return Config
}
