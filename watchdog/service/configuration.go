package service

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"go-toolkit/watchdog/product"
	"log"
	"os"
)

type Config struct {
	Funds  map[string]*product.Fund  `yaml:"funds"`
	Stocks map[string]*product.Stock `yaml:"stocks"`
	SiftAI struct {
		Endpoint  string `yaml:"endpoint"`
		Model     string `yaml:"model"`
		OutputDir string `yaml:"output-dir"`
	} `yaml:"sift-ai"`
	Token struct {
		Lark     string `yaml:"lark"`
		DingTalk string `yaml:"dingtalk"`
	} `yaml:"token"`
}

const (
	DefaultSiftAIEndpoint  = "https://api.openai.com"
	DefaultSiftAIModel     = "gpt-4o-mini"
	DefaultSiftAIOutputDir = "./sift-results"
)

var ConfigTemplate = fmt.Sprintf(`
funds:
  008099: # 基金代码
    cost: 1.6078 # 基金成本价
  000083: 
    cost: 5.1727

stocks:
  510210.SH: # 股票代码
    low: 0.7 # 监控阈值低点 
    high: 1.0 # 监控阈值高点

token:
  lark: xxxxxx # 飞书机器人 Webhook token，可选
  dingtalk: xxxxxx # 钉钉机器人 Webhook token，可选

sift-ai:
  endpoint: https://api.openai.com
  model: gpt-4o-mini
  output-dir: ./sift-results`)

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
	fillSiftAIDefaults(&config)
	return &config
}

func fillSiftAIDefaults(config *Config) {
	if config.SiftAI.Endpoint == "" {
		config.SiftAI.Endpoint = DefaultSiftAIEndpoint
	}
	if config.SiftAI.Model == "" {
		config.SiftAI.Model = DefaultSiftAIModel
	}
	if config.SiftAI.OutputDir == "" {
		config.SiftAI.OutputDir = DefaultSiftAIOutputDir
	}
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
