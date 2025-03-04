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

	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "chat-llms",
		Usage:   "Chat with multi-LLMs at the same time",
		Version: "v2.3.1",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "repeat",
				Aliases:  []string{"r"},
				Usage:    "Repeat times of every temperature, default 3",
				Value:    3,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "output-folder",
				Aliases:  []string{"o"},
				Usage:    "Folder of output txt files, default ./result",
				Value:    "./result",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "system-prompt",
				Aliases:  []string{"sp"},
				Usage:    "System prompt txt file path, default ./system_prompt.txt",
				Value:    "./system_prompt.txt",
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			systemPromptFilePath := cCtx.String("system-prompt")
			repeatTimes := cCtx.Int("repeat")
			outputFolder := cCtx.String("output-folder")

			err := os.MkdirAll(outputFolder, 0755)
			if err != nil {
				return fmt.Errorf("创建文件夹失败: %v", err)
			}

			doChat(systemPromptFilePath, repeatTimes, outputFolder)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func doChat(systemPromptFilePath string, repeatTimes int, outputFolder string) {
	systemPrompt, err := readSystemPrompt(systemPromptFilePath)
	if err != nil {
		fmt.Println("读取系统提示词失败:", err)
		return
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
			for _, temp := range m.Temperatures {
				for i := 0; i < repeatTimes; i++ {
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
				}
			}
		}(id, model)
	}

	wg.Wait()
	fmt.Println("所有模型调用完成！")
}

func callAPI(id string, model ModelConfig, temperature float64, isStream bool, systemPrompt string, chatHistory []ChatMessage, idx int, outputFolder string) error {
	messages := make([]Message, 0)
	messages = append(messages, Message{Role: "system", Content: systemPrompt})
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

	suffix := ""
	if isStream {
		suffix = "_stream"
	}
	fileName := fmt.Sprintf("%s/%s_%v%s_%d.txt", outputFolder, id, temperature, suffix, idx)
	err = os.WriteFile(fileName, []byte(fmt.Sprintf("%s\r\n\r\n%s", fileName, content)), 0644)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	fmt.Printf("第 %d 次调用模型 %s 温度 %.2f%s 成功，结果已保存到 %s\n", idx+1, id, temperature, suffix, fileName)

	return nil
}

func readSystemPrompt(systemPromptFilePath string) (string, error) {
	content, err := os.ReadFile(systemPromptFilePath)
	if err != nil {
		return "", fmt.Errorf("读取系统提示词失败: %v", err)
	}
	return string(content), nil
}

func readChatHistory() ([]ChatMessage, error) {
	content, err := os.ReadFile("chat_history.json")
	if err != nil {
		return nil, fmt.Errorf("读取对话记录失败: %v", err)
	}
	var messages []ChatMessage
	err = json.Unmarshal(content, &messages)
	if err != nil {
		return nil, fmt.Errorf("解析对话记录失败: %v", err)
	}
	return messages, nil
}

func readModelConfig() (*Config, error) {
	content, err := os.ReadFile("models_config.yaml")
	if err != nil {
		return nil, fmt.Errorf("读取模型配置失败: %v", err)
	}
	var config Config
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, fmt.Errorf("解析模型配置失败: %v", err)
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
