/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - config file parse engine
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package utility

import (
	"errors"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
)

// Config .
type Config struct {
	data map[string]string
}

// NewConfig .
func NewConfig() *Config {
	return &Config{data: make(map[string]string)}
}

const emptyRunes = " \r\t\v"

// Load .
func (c *Config) Load(configFile string) error {
	stream, err := ioutil.ReadFile(configFile)
	if err != nil {
		return errors.New("cannot load config file")
	}
	content := string(stream)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.Trim(line, emptyRunes)
		// 去除空行和注释
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			for i, part := range parts {
				parts[i] = strings.Trim(part, emptyRunes)
			}
			c.data[parts[0]] = parts[1]
		} else {
			// 判断并处理include条目，load相应的config文件
			// include 的配置文件应该在当前配置文件所在的目录下
			includes := strings.SplitN(parts[0], " ", 2)
			if len(includes) == 2 && strings.EqualFold(includes[0], "include") {
				// 拼解新包含config文件的path
				confDir := path.Dir(configFile)
				newConfName := strings.Trim(includes[1], emptyRunes)
				newConfPath := path.Join(confDir, newConfName)
				// 载入include的config文件，调用Load自身
				if err := c.Load(newConfPath); err != nil {
					return errors.New("load include config file failed")
				}
				continue
			} else {
				return errors.New("invalid config file syntax")
			}
		}
	}
	return nil
}

// GetAll .
func (c *Config) GetAll() map[string]string {
	return c.data
}

// Get .
func (c *Config) Get(key string) string {
	if value, ok := c.data[key]; ok {
		return value
	}
	return ""
}

// GetInt .
func (c *Config) GetInt(key string) int {
	value := c.Get(key)
	if value == "" {
		return 0
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return result
}

// GetInt64 .
func (c *Config) GetInt64(key string) int64 {
	value := c.Get(key)
	if value == "" {
		return 0
	}
	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return result
}

// GetSlice .
func (c *Config) GetSlice(key string, separator string) []string {
	var slice []string
	value := c.Get(key)
	if value != "" {
		for _, part := range strings.Split(value, separator) {
			slice = append(slice, strings.Trim(part, emptyRunes))
		}
	}
	return slice
}

// GetSliceInt .
func (c *Config) GetSliceInt(key string, separator string) []int {
	slice := c.GetSlice(key, separator)
	var results []int
	for _, part := range slice {
		result, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		results = append(results, result)
	}
	return results
}
