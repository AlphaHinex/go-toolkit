package service

import (
	"github.com/go-yaml/yaml"
	"go-toolkit/watchdog/product"
	"log"
	"os"
)

type Config struct {
	Funds  map[string]*product.Fund  `yaml:"funds"`
	Stocks map[string]*product.Stock `yaml:"stocks"`
	Token  struct {
		Lark     string `yaml:"lark"`
		DingTalk string `yaml:"dingtalk"`
	} `yaml:"token"`
}

func ReadConfigs(configsFilePath string) *Config {
	content, err := os.ReadFile(configsFilePath)
	if err != nil {
		log.Panicf("读取配置 %s 失败: %v", configsFilePath, err)
	}
	var config Config
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		log.Panicf("解析配置失败: %v", err)
	}
	return &config
}

func WriteConfigs(configFilePath string, configs *Config) {
	// 将 Configs 内容序列化至文件
	content, err := yaml.Marshal(configs)
	if err != nil {
		log.Panicf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(configFilePath, content, 0644); err != nil {
		log.Panicf("写入配置文件 %s 失败: %v", configFilePath, err)
	}
}
