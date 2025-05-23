package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
)

var systemPromptTemplate = `请在此处填写系统提示词。
可多行。
也可以为空。
`

var chatHistoryTemplate = `[
    {
        "role": "user",
        "content": "此处内容代表用户的第一个问题。"
    },
    {
        "role": "assistant",
        "content": "此处代表对上一个用户问题的回答。"
    },
    {
        "role": "user",
        "content": "需以用户角色的内容结尾，代表用户最后的问题（即至少要有一个用户角色的内容）。"
    }
]
`

var modelsConfigTemplate = `模型1_ID:
  endpoint: https://api.openai.com
  api-key: sk-xxxxxxxx
  model: text-davinci-003
  temperatures:
    - 0.5
    - 0.7
    - 0.9
  enabled: true
模型2_ID:
  endpoint: https://api.openai.com
  api-key: sk-xxxxxxxx
  model: GPT-4o
  temperatures:
    - 0.5
    - 0.7
    - 0.9
  enabled: false
`

var modelsConfigFilePath = ""
var systemPromptFilePath = ""
var chatHistoryFilePath = ""
var repeatTimes = 0
var parallel = 0
var outputFolder = ""

func main() {
	app := &cli.App{
		Name:    "chat-llms",
		Usage:   "Chat with multi-LLMs at the same time.",
		Version: "v2.5.0",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "repeat",
				Aliases:  []string{"r"},
				Usage:    "Repeat times of every temperature, default is 3 .",
				Value:    3,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "output-folder",
				Aliases:  []string{"o"},
				Usage:    "Folder of output txt files, default is ./result .",
				Value:    "./result",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "system-prompt",
				Aliases:  []string{"s"},
				Usage:    "System prompt txt file path, default is ./system_prompt.txt .",
				Value:    "./system_prompt.txt",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "chat-history",
				Aliases:  []string{"u"},
				Usage:    "Chat history with user input content JSON file path, default is ./chat_history.json .",
				Value:    "./chat_history.json",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "models-config",
				Aliases:  []string{"c"},
				Usage:    "LLM models config file path, default is ./models_config.yaml .",
				Value:    "./models_config.yaml",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "templates",
				Aliases:  []string{"t"},
				Usage:    "Generate template files of system_prompt.txt, chat_history.json and models_config.yaml in current path.",
				Value:    false,
				Required: false,
			},
			&cli.IntFlag{
				Name:     "parallel",
				Aliases:  []string{"p"},
				Usage:    "Parallel of chat request, default is 1.",
				Value:    1,
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			needTemplates := cCtx.Bool("templates")
			modelsConfigFilePath = cCtx.String("models-config")
			systemPromptFilePath = cCtx.String("system-prompt")
			chatHistoryFilePath = cCtx.String("chat-history")
			repeatTimes = cCtx.Int("repeat")
			parallel = cCtx.Int("parallel")
			outputFolder = cCtx.String("output-folder")

			if needTemplates {
				err1 := os.WriteFile("system_prompt.txt_template", []byte(systemPromptTemplate), 0644)
				err2 := os.WriteFile("chat_history.json_template", []byte(chatHistoryTemplate), 0644)
				err3 := os.WriteFile("models_config.yaml_template", []byte(modelsConfigTemplate), 0644)
				if err1 != nil || err2 != nil || err3 != nil {
					return fmt.Errorf("生成模板文件失败: %v, %v, %v", err1, err2, err3)
				} else {
					fmt.Println("生成模板文件成功！")
				}
			} else {
				err := os.MkdirAll(outputFolder, 0755)
				if err != nil {
					return fmt.Errorf("创建文件夹失败: %v", err)
				}

				doChat()
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func doChat() {
	systemPrompt, err := readSystemPrompt()
	if err != nil {
		fmt.Println("未读取到系统提示词，请求中将不会添加。如需使用系统提示词，可通过 system_prompt.txt 文件设置。")
	}

	chatHistory, err := readChatHistory()
	if err != nil {
		fmt.Println("读取对话记录失败:", err)
		return
	}

	config, err := readModelConfig()
	if err != nil {
		fmt.Println("读取模型配置失败:", err)
		return
	}

	for id, model := range config.Models {
		if !model.Enabled {
			continue
		}
		fmt.Printf("模型 %s 已启用，温度范围: %v\n", id, model.Temperatures)
	}

	var wg sync.WaitGroup
	for id, model := range config.Models {
		if !model.Enabled {
			continue
		}
		wg.Add(1)
		go func(id string, m ModelConfig) {
			defer wg.Done()
			var wgTemp sync.WaitGroup
			sem := make(chan struct{}, parallel) // parallel 指定并发数

			for _, temp := range m.Temperatures {
				for i := 0; i < repeatTimes; i++ {
					wgTemp.Add(1)
					sem <- struct{}{} // 获取信号量
					go func(temp float64, i int) {
						defer wgTemp.Done()
						defer func() { <-sem }() // 释放信号量

						fmt.Printf("调用模型 %s 温度 %.2f 非流式接口第 %d 次……\n", id, temp, i+1)
						// 调用普通接口
						err := callAPI(id, m, temp, false, systemPrompt, chatHistory, i, outputFolder)
						if err != nil {
							fmt.Printf("调用模型 %s 温度 %.2f 非流式接口失败: %v\n", id, temp, err)
						}
						fmt.Printf("调用模型 %s 温度 %.2f 流式接口第 %d 次……\n", id, temp, i+1)
						// 调用流式接口
						err = callAPI(id, m, temp, true, systemPrompt, chatHistory, i, outputFolder)
						if err != nil {
							fmt.Printf("调用模型 %s 温度 %.2f 流式接口失败: %v\n", id, temp, err)
						}
					}(temp, i)
				}
			}
			wgTemp.Wait()
		}(id, model)
	}

	wg.Wait()
	fmt.Println("所有模型调用完成！")
}

func callAPI(id string, model ModelConfig, temperature float64, isStream bool, systemPrompt string, chatHistory []ChatMessage, idx int, outputFolder string) error {
	messages := make([]Message, 0)
	if len(systemPrompt) > 0 {
		messages = append(messages, Message{Role: "system", Content: systemPrompt})
	}
	for _, msg := range chatHistory {
		messages = append(messages, Message{Role: msg.Role, Content: msg.Content})
	}

	body := map[string]interface{}{
		"messages":    messages,
		"model":       model.Model,
		"temperature": temperature,
	}

	if isStream {
		body["stream"] = true
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %v", err)
	}

	start := time.Now()
	client := &http.Client{}
	req, err := http.NewRequest("POST", model.Endpoint+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+model.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("API返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var content string
	if isStream {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				jsonStr := strings.TrimPrefix(line, "data: ")
				if jsonStr == "[DONE]" {
					break
				}
				var delta map[string]interface{}
				err := json.Unmarshal([]byte(jsonStr), &delta)
				if err != nil {
					continue
				}
				deltaContent, ok := delta["choices"].([]interface{})[0].(map[string]interface{})["delta"].(map[string]interface{})["content"].(string)
				if ok {
					content += deltaContent
				}
			}
		}
	} else {
		var apiResp APIResponse
		err := json.NewDecoder(resp.Body).Decode(&apiResp)
		if err != nil {
			return fmt.Errorf("解析响应失败: %v", err)
		}
		if len(apiResp.Choices) > 0 {
			content = apiResp.Choices[0].Message.Content
		}
	}
	elapsed := time.Since(start).Milliseconds()
	suffix := ""
	if isStream {
		suffix = "_stream"
	}
	fileName := fmt.Sprintf("%s/%s_%v%s_%d.txt", outputFolder, id, temperature, suffix, idx)
	err = os.WriteFile(fileName, []byte(fmt.Sprintf("%s\t%dms\r\n\r\n%s", fileName, elapsed, content)), 0644)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	fmt.Printf("第 %d 次调用模型 %s 温度 %.2f%s 成功，结果已保存到 %s\n", idx+1, id, temperature, suffix, fileName)

	return nil
}

func readSystemPrompt() (string, error) {
	content, err := os.ReadFile(systemPromptFilePath)
	if err != nil {
		return "", fmt.Errorf("读取系统提示词失败: %v", err)
	}
	fileName := fmt.Sprintf("%s/system_prompt.txt", outputFolder)
	err = os.WriteFile(fileName, content, 0644)
	if err != nil {
		return "", fmt.Errorf("备份系统提示词文件失败: %v", err)
	}
	return string(content), nil
}

func readChatHistory() ([]ChatMessage, error) {
	content, err := os.ReadFile(chatHistoryFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取对话记录失败: %v", err)
	}
	var messages []ChatMessage
	err = json.Unmarshal(content, &messages)
	if err != nil {
		return nil, fmt.Errorf("解析对话记录失败: %v", err)
	}
	fileName := fmt.Sprintf("%s/chat_history.json", outputFolder)
	err = os.WriteFile(fileName, content, 0644)
	if err != nil {
		return nil, fmt.Errorf("备份对话历史文件失败: %v", err)
	}
	return messages, nil
}

func readModelConfig() (*Config, error) {
	content, err := os.ReadFile(modelsConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取模型配置失败: %v", err)
	}
	var config Config
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, fmt.Errorf("解析模型配置失败: %v", err)
	}
	fileName := fmt.Sprintf("%s/models_config.yaml", outputFolder)
	err = os.WriteFile(fileName, content, 0644)
	if err != nil {
		return nil, fmt.Errorf("备份模型配置文件失败: %v", err)
	}
	return &config, nil
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelConfig struct {
	Endpoint     string    `yaml:"endpoint"`
	ApiKey       string    `yaml:"api-key"`
	Model        string    `yaml:"model"`
	Temperatures []float64 `yaml:"temperatures"`
	Enabled      bool      `yaml:"enabled"`
}

// Config 表示整个 YAML 文件的结构
type Config struct {
	Models map[string]ModelConfig `yaml:",inline"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
