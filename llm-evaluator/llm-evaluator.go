package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var modelsConfigTemplate = `candidate:
  endpoint: https://api.openai.com
  api-key: sk-xxxxxxxx
  model: text-davinci-003
  temperature: 0
evaluator:
  endpoint: https://api.openai.com
  api-key: sk-xxxxxxxx
  model: GPT-4o
  temperature: 0
`

var inputFilePath = ""
var modelsConfigFilePath = ""
var parallel = 0
var outputFolder = ""

func main() {
	app := &cli.App{
		Name:    "llm-evaluator",
		Usage:   "Evaluate QA capability of LLM model with LLM model.",
		Version: "v2.4.1",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input-file",
				Aliases:  []string{"i"},
				Usage:    "CSV file with `q` (as question) and `a` (as answer) columns, default is ./input.csv .",
				Value:    "./input.csv",
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
				Name:     "models-config",
				Aliases:  []string{"c"},
				Usage:    "LLM models config file path, default is ./models_config.yaml .",
				Value:    "./models_config.yaml",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "templates",
				Aliases:  []string{"t"},
				Usage:    "Generate template files of models_config.yaml in current path.",
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
			inputFilePath = cCtx.String("input-file")
			needTemplates := cCtx.Bool("templates")
			modelsConfigFilePath = cCtx.String("models-config")
			parallel = cCtx.Int("parallel")
			outputFolder = cCtx.String("output-folder")

			if needTemplates {
				err := os.WriteFile("models_config.yaml_template", []byte(modelsConfigTemplate), 0644)
				if err != nil {
					return fmt.Errorf("生成模板文件失败: %v", err)
				} else {
					fmt.Println("生成模板文件成功！")
				}
			} else {
				err := os.MkdirAll(outputFolder, 0755)
				if err != nil {
					return fmt.Errorf("创建文件夹失败: %v", err)
				}

				doEvaluate()
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func doEvaluate() {
	config, err := readModelConfig()
	if err != nil {
		fmt.Println("读取模型配置失败:", err)
		return
	}
	qa, err := readInputCSV()
	if err != nil {
		fmt.Println("读取输入文件失败:", err)
		return
	}
	// 从 qa[0] 中查找 q 列索引
	var qIndex, aIndex, sIndex int
	for i, header := range qa[0] {
		if strings.ToLower(header) == "q" {
			qIndex = i
		} else if strings.ToLower(header) == "a" {
			aIndex = i
		} else if strings.ToLower(header) == "s" {
			sIndex = i
		}
	}
	if qIndex == 0 || aIndex == 0 || sIndex == 0 {
		fmt.Println("输入文件格式错误，必须包含 q、a 和 s 列")
		return
	}

	candidateModel := config.Models["candidate"]
	evaluatorModel := config.Models["evaluator"]

	// 定义 channel
	results := make(chan string)
	// 定义一个带缓冲的 channel 作为信号量
	semaphore := make(chan struct{}, parallel) // parallel 是并发限制数量

	var wg sync.WaitGroup
	for _, record := range qa[1:] { // 跳过表头
		wg.Add(1)
		go func(record []string) {
			defer wg.Done()

			// 占用一个并发槽
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // 释放并发槽

			// 获取问题和标准答案
			question := record[qIndex]
			expectedAnswer := record[aIndex]
			evaluateStandard := record[sIndex]
			// 调用候选模型作答
			answer, err := callChatAPI(candidateModel, true, question)
			answer = cleanThinkOfDeepSeek(answer)
			if err != nil {
				fmt.Printf("调用模型 %s 失败: %v\n", candidateModel.Model, err)
				return
			}

			score := "-2"
			scoreWithReason := ""
			if evaluateStandard == "=" {
				// 判断 answer 与 expectedAnswer 是否相等
				if strings.TrimSpace(answer) == strings.TrimSpace(expectedAnswer) {
					score = "1"
					scoreWithReason = "与标准答案安全一致"
				} else {
					score = "0"
					scoreWithReason = "与标准答案不完全一致"
				}
			} else {
				// 调用评估模型
				scoreWithReason, err = callChatAPI(evaluatorModel, true, getEvaluatePrompt(question, answer, expectedAnswer))
				score = cleanThinkOfDeepSeek(scoreWithReason)
				if err != nil {
					fmt.Printf("调用模型 %s 失败: %v\n", evaluatorModel.Model, err)
					return
				}
			}
			results <- fmt.Sprintf("%s,%s,%s,%s", strings.Join(record, ","), toOneLine(answer), toOneLine(score), toOneLine(scoreWithReason))
			fmt.Printf("放入 channel\n")
		}(record)
	}

	// 启动一个 goroutine 关闭 channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// 将 channel 中的内容写入文件
	outputFilePath := fmt.Sprintf("%s/results.csv", outputFolder)
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		log.Fatalf("创建输出文件失败: %v", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	_, err = writer.WriteString(fmt.Sprintf("%s,answer,score,reason\n", strings.Join(qa[0], ",")))
	if err != nil {
		log.Fatalf("写入文件失败: %v", err)
	}
	for result := range results {
		_, err = writer.WriteString(result + "\n")
		if err != nil {
			log.Fatalf("写入文件失败: %v", err)
		}
	}
	writer.Flush()
	fmt.Printf("结果已写入文件: %s\n", outputFilePath)
}

func toOneLine(answer string) interface{} {
	// 将回答内容转换为单行
	return strings.ReplaceAll(strings.TrimSpace(answer), "\n", " ")
}

func getEvaluatePrompt(question string, answer string, expectedAnswer string) string {
	return fmt.Sprintf("请根据问题和标准答案，评估回答内容是否正确。正确返回`1`，错误返回`0`，不确定返回`-1`。\n问题: %s\n标准答案: %s\n回答: %s", question, expectedAnswer, answer)
}

func callChatAPI(model ModelConfig, isStream bool, userPrompt string) (string, error) {
	fmt.Printf("调用模型 %s %s，温度 %.2f，流式: %t\n", model.Endpoint, model.Model, model.Temperature, isStream)
	messages := make([]Message, 0)
	messages = append(messages, Message{Role: "user", Content: userPrompt})

	body := map[string]interface{}{
		"user":        "llm-evaluator",
		"messages":    messages,
		"model":       model.Model,
		"temperature": model.Temperature,
	}

	if isStream {
		body["stream"] = true
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %v", err)
	}

	client := &http.Client{
		Timeout: 600 * time.Second, // 设置超时时间为 600 秒
	}
	req, err := http.NewRequest("POST", model.Endpoint+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+model.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now() // 记录开始时间
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("API返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var content string
	if isStream {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			fmt.Print(".")
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
			return "", fmt.Errorf("解析响应失败: %v", err)
		}
		if len(apiResp.Choices) > 0 {
			content = apiResp.Choices[0].Message.Content
		}
	}
	duration := time.Since(start) // 计算调用时长
	fmt.Printf("\n模型输出：\n%s\n", content)
	fmt.Printf("\n调用耗时 %v (%s~) \n", duration, start)
	return content, nil
}

func cleanThinkOfDeepSeek(content string) string {
	// 定义多行匹配的正则表达式
	// (?s) 启用单行模式，使 . 可以匹配换行符
	re := regexp.MustCompile(`(?s)(<think>)?.*?</think>`)

	// 替换匹配的内容
	return strings.TrimSpace(re.ReplaceAllString(content, ""))
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

func readInputCSV() ([][]string, error) {
	file, err := os.Open(inputFilePath)
	if err != nil {
		return nil, fmt.Errorf("打开输入文件失败: %v", err)
	}
	defer file.Close()

	// 创建 CSV 读取器
	reader := csv.NewReader(file)
	// 读取所有行
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Printf("读取输入文件失败: %v\n", err)
		return nil, err
	}
	return records, nil
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelConfig struct {
	Endpoint    string  `yaml:"endpoint"`
	ApiKey      string  `yaml:"api-key"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
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
