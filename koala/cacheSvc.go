/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Redis cache conn pool
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package koala

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

// InitRedisPool redis连接池初始化函数
func InitRedisPool() {
	// redis服务 host:port
	var server string = Config.Get("redis_server")
	// redis服务 口令
	var password string = Config.Get("redis_auth")
	// redis连接池 最大空闲连接数
	var maxIdle int = Config.GetInt("redis_pool_maxIdle")
	// redis连接池 空闲连接超时时长
	var idleTimeout int = Config.GetInt("redis_pool_idleTimeout")

	var connectTimeout time.Duration = time.Duration(Config.GetInt("externalConnTimeout")) * time.Millisecond
	var readTimeout time.Duration = time.Duration(Config.GetInt("externalReadTimeout")) * time.Millisecond
	var writeTimeout time.Duration = time.Duration(Config.GetInt("externalWriteTimeout")) * time.Millisecond

	// 新建连接池
	RedisPool = &redis.Pool{
		MaxIdle:     maxIdle,
		IdleTimeout: time.Duration(idleTimeout) * time.Second,
		// dial方法
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server, redis.DialConnectTimeout(connectTimeout), redis.DialReadTimeout(readTimeout), redis.DialReadTimeout(writeTimeout))
			if err != nil {
				return nil, err
			}
			if _, err = c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
			return c, err
		},
		// 连接可用性检查 方法
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
