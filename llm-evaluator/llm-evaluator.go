package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var evaluatorPrompt = `
    ## 目标
    
    请根据问题和标准答案，评估回答的内容与标准答案中内容是否存在本质上的区别，并给出评估依据。以 json 结构返回评估结果，score 代表得分，reason 代表原因。
    无区别 score 为 1，有区别为 0，不确定为 -1。
    
    ## 返回结构示例
    
    {"score":"1", "reason":"给出评分依据"}
    
    ## 评估内容 
    
    ### 问题: 
    
    {question}
    
    ### 标准答案: 
    
    {expectedAnswer}
    
    ### 回答: 
    
    {answer}
`

var configsTemplate = fmt.Sprintf(`
# 必填
model:
  # 运动员
  candidate:
    endpoint: https://api.openai.com
    api-key: sk-xxxxxxxx
    model: text-davinci-003
    temperature: 0
  # 裁判员
  evaluator:
    endpoint: https://api.openai.com
    api-key: sk-xxxxxxxx
    model: GPT-4o
    temperature: 0

# 必填
input:
  file: ./input.csv
  # 问题列名
  question: question
  # 标准答案列名
  expectedAnswer: expectedAnswer
  # 实际回答列名
  answer: answer

# 可使用默认值
output:
  folder: ./result

# 可使用默认值
prompt:
  # 提示词中的 {question}、{expectedAnswer}、{answer} 分别会被替换为 问题、标准答案、实际回答内容
  evaluator: |
%s

# 可选
langfuse:
  enable: false
  host: https://cloud.langfuse.com
  public-key: pk-lf-xxx
  secret-key: sk-lf-xxx
`, evaluatorPrompt)

var outputFolder = ""
var parallel = 0

func main() {
	app := &cli.App{
		Name:    "llm-evaluator",
		Usage:   "Evaluate QA capability of LLM model with LLM model.",
		Version: "v2.4.2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "configs",
				Aliases:  []string{"c"},
				Usage:    "LLM Evaluator configs file path.",
				Value:    "./configs.yaml",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "templates",
				Aliases:  []string{"t"},
				Usage:    "Generate template files of configs.yaml in current path.",
				Value:    false,
				Required: false,
			},
			&cli.IntFlag{
				Name:     "parallel",
				Aliases:  []string{"p"},
				Usage:    "Parallel of chat request.",
				Value:    1,
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			needTemplates := cCtx.Bool("templates")
			configsFilePath := cCtx.String("configs")
			parallel = cCtx.Int("parallel")

			if needTemplates {
				err := os.WriteFile("configs.yaml_template", []byte(configsTemplate), 0644)
				if err != nil {
					return fmt.Errorf("生成模板文件失败: %v", err)
				} else {
					fmt.Println("生成模板文件成功！")
				}
			} else {
				configs := readConfigs(configsFilePath)
				outputFolder = configs.Output.Folder
				if strings.TrimSpace(outputFolder) == "" {
					// 使用默认输出路径
					outputFolder = "./result"
				}
				backupConfigsToOutputFolder(configs, outputFolder)

				doEvaluate(configs)
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func doEvaluate(configs *Configs) {
	qa, err := readInputCSV(configs.Input.File)
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
	if qIndex < 0 || aIndex < 0 {
		fmt.Println("输入文件格式错误，必须包含 q 和 a 列")
		return
	}

	candidateModel := configs.Model.Candidate
	evaluatorModel := configs.Model.Evaluator

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

			var questions []string
			chatMessages, err := parseChatMessages(question)
			if err != nil {
				questions = append(questions, question)
			} else {
				for _, message := range chatMessages {
					if message.Role == "user" {
						questions = append(questions, message.Content)
					}
				}
			}

			// 多轮对话 id 取最后一个
			var id, answer string
			var chatHistory []Message
			for _, q := range questions {
				// 调用候选模型作答
				id, answer, err = callChatAPI(candidateModel, true, q, chatHistory)
				if err != nil {
					fmt.Printf("调用模型 %s 失败: %v\n", candidateModel.Model, err)
				}
				answer = cleanThinkOfDeepSeek(answer)
				chatHistory = append(chatHistory, Message{
					Role:    "user",
					Content: q,
				}, Message{
					Role:    "assistant",
					Content: answer,
				})
			}

			if len(chatHistory) == 2 {
				answer = chatHistory[1].Content
			} else {
				// 将 chatHistory 转为 json 字符串
				chatHistoryJSON, err := json.Marshal(chatHistory)
				if err != nil {
					fmt.Printf("序列化聊天记录失败: %v\n", err)
					return
				}
				answer = string(chatHistoryJSON)
			}

			score := ""
			reason := ""
			if sIndex > 0 && record[sIndex] == "=" {
				// 判断 answer 与 expectedAnswer 是否完全一致
				if strings.TrimSpace(answer) == strings.TrimSpace(expectedAnswer) {
					score = "1"
					reason = "与标准答案安全一致"
				} else {
					score = "0"
					reason = "与标准答案不完全一致"
				}
			} else {
				// 调用评估模型
				_, scoreWithReason, err := callChatAPI(evaluatorModel, true, getEvaluatePrompt(configs.Prompt.Evaluator, question, answer, expectedAnswer), nil)
				scoreWithReason = cleanThinkOfDeepSeek(scoreWithReason)
				scoreWithReason = cleanMarkdownJsonSymbolIfNeeded(scoreWithReason)
				// 将 scoreWithReason 转成 json
				var result map[string]string
				err = json.Unmarshal([]byte(scoreWithReason), &result)
				score = result["score"]
				reason = result["reason"]
				if err != nil {
					fmt.Printf("调用模型 %s 失败: %v\n", evaluatorModel.Model, err)
					return
				}
			}
			results <- fmt.Sprintf("%s,%s,%s,%s", strings.Join(toOneCells(record), ","), toOneCell(answer), toOneCell(score), toOneCell(reason))
			if configs.Langfuse.Enable {
				createLangfuseScore(configs, id, score, question, answer, expectedAnswer, reason)
			}
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

func createLangfuseScore(configs *Configs, id string, score string, question string, answer string, expectedAnswer string, reason string) {
	var value interface{}
	scoreInt, err := strconv.Atoi(score)
	if err != nil {
		value = score
	} else {
		value = scoreInt
	}
	body := LangfuseScore{
		TraceId: id,
		Value:   value,
		Name:    configs.Langfuse.ScoreName,
		Metadata: struct {
			Reason         string `json:"reason"`
			Answer         string `json:"answer"`
			Question       string `json:"question"`
			ExpectedAnswer string `json:"expected_answer"`
		}{
			Reason:         reason,
			Answer:         answer,
			Question:       question,
			ExpectedAnswer: expectedAnswer,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		fmt.Printf("序列化 Langfuse Score 请求体异常 %s", err)
	}
	req, err := http.NewRequest("POST", configs.Langfuse.Host+"/api/public/scores", bytes.NewBuffer(jsonBody))
	req.SetBasicAuth(configs.Langfuse.PublicKey, configs.Langfuse.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("创建 Langfuse Score 请求异常 %s", err)
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	if statusCode != 200 {
		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("读取创建 Langfuse Score 请求响应体异常 %s", err)
		}
		fmt.Printf("调用 Langfuse 失败：%s %s\n", resp.Status, string(body))
	}
}

func toOneCells(contents []string) []string {
	for i, content := range contents {
		contents[i] = toOneCell(content)
	}
	return contents
}

func toOneCell(content string) string {
	if strings.Contains(content, ",") || strings.Contains(content, "\n") {
		content = fmt.Sprintf("\"%s\"", strings.ReplaceAll(strings.TrimSpace(content), "\"", "\"\""))
	}
	return content
}

func getEvaluatePrompt(prompt, question, answer, expectedAnswer string) string {
	if strings.TrimSpace(prompt) == "" {
		prompt = evaluatorPrompt
	}
	prompt = strings.ReplaceAll(prompt, "{question}", question)
	prompt = strings.ReplaceAll(prompt, "{expectedAnswer}", expectedAnswer)
	prompt = strings.ReplaceAll(prompt, "{answer}", answer)
	return prompt
}

func callChatAPI(model ModelConfig, isStream bool, userPrompt string, history []Message) (string, string, error) {
	fmt.Printf("调用模型 %s %s，温度 %.2f，流式: %t\n", model.Endpoint, model.Model, model.Temperature, isStream)
	messages := make([]Message, len(history)+1)
	messages = append(history, Message{Role: "user", Content: userPrompt})

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
		return "", "", fmt.Errorf("序列化请求体失败: %v", err)
	}

	client := &http.Client{
		Timeout: 600 * time.Second, // 设置超时时间为 600 秒
	}
	req, err := http.NewRequest("POST", model.Endpoint+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+model.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now() // 记录开始时间
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", "", fmt.Errorf("API返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var id, content string
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
				var apiResp StreamingAPIResponse
				err := json.Unmarshal([]byte(jsonStr), &apiResp)
				if err != nil {
					return "", "", fmt.Errorf("解析响应失败: %v", err)
				}
				if len(apiResp.Choices) > 0 {
					content += apiResp.Choices[0].Delta.Content
					id = apiResp.Id
				}
			}
		}
	} else {
		var apiResp BlockingAPIResponse
		err := json.NewDecoder(resp.Body).Decode(&apiResp)
		if err != nil {
			return "", "", fmt.Errorf("解析响应失败: %v", err)
		}
		if len(apiResp.Choices) > 0 {
			content = apiResp.Choices[0].Message.Content
			id = apiResp.Id
		}
	}
	duration := time.Since(start) // 计算调用时长
	fmt.Printf("\n模型输出：\n%s\n", content)
	fmt.Printf("\n调用耗时 %v (%s~) \n", duration, start)
	return id, content, nil
}

func cleanThinkOfDeepSeek(content string) string {
	// 定义多行匹配的正则表达式
	// (?s) 启用单行模式，使 . 可以匹配换行符
	re := regexp.MustCompile(`(?s)(<think>)?.*?</think>`)

	// 替换匹配的内容
	return strings.TrimSpace(re.ReplaceAllString(content, ""))
}

func cleanMarkdownJsonSymbolIfNeeded(content string) string {
	idx := strings.Index(content, "```json")
	if idx > -1 {
		content = content[idx+7:]
	}
	if strings.HasSuffix(content, "```") {
		content = content[:len(content)-3]
	}
	return content
}

func readConfigs(configsFilePath string) *Configs {
	content, err := os.ReadFile(configsFilePath)
	if err != nil {
		log.Panicf("读取模型配置 %s 失败: %v", configsFilePath, err)
	}
	var config Configs
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		log.Panicf("解析模型配置失败: %v", err)
	}
	return &config
}

func backupConfigsToOutputFolder(configs *Configs, outputFolder string) {
	err := os.MkdirAll(outputFolder, 0755)
	if err != nil {
		log.Panicf("创建输出文件夹失败: %v", err)
	}
	fileName := fmt.Sprintf("%s/configs.yaml", outputFolder)
	content, err := yaml.Marshal(configs)
	if err != nil {
		log.Panicf("序列化模型配置失败: %v", err)
	}
	// 将内容写入文件
	err = os.WriteFile(fileName, content, 0644)
	if err != nil {
		log.Panicf("备份模型配置文件失败: %v", err)
	}
}

func readInputCSV(inputFilePath string) ([][]string, error) {
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

func parseChatMessages(s string) ([]Message, error) {
	var messages []Message
	err := json.Unmarshal([]byte(s), &messages)
	if err != nil {
		return nil, err
	}
	return messages, nil
}

type ModelConfig struct {
	Endpoint    string  `yaml:"endpoint"`
	ApiKey      string  `yaml:"api-key"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
}

// Configs 表示整个 YAML 文件的结构
type Configs struct {
	Model struct {
		Candidate ModelConfig `yaml:"candidate"`
		Evaluator ModelConfig `yaml:"evaluator"`
	} `yaml:"model"`
	Input struct {
		File           string `yaml:"file"`
		Question       string `yaml:"question"`
		ExpectedAnswer string `yaml:"expectedAnswer"`
		Answer         string `yaml:"answer"`
	} `yaml:"input"`
	Output struct {
		Folder string `yaml:"folder"`
	} `yaml:"output"`
	Prompt struct {
		Evaluator string `yaml:"evaluator"`
	} `yaml:"prompt"`
	Langfuse struct {
		Enable    bool   `yaml:"enable"`
		Host      string `yaml:"host"`
		PublicKey string `yaml:"public-key"`
		SecretKey string `yaml:"secret-key"`
		ScoreName string `yaml:"score-name"`
	} `yaml:"langfuse"`
}

// Message 请求消息结构
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BlockingAPIResponse 非流式响应消息结构
type BlockingAPIResponse struct {
	Id      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// StreamingAPIResponse 流式响应消息结构
type StreamingAPIResponse struct {
	Id      string `json:"id"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type LangfuseScore struct {
	Name     string      `json:"name"`
	Value    interface{} `json:"value"`
	TraceId  string      `json:"traceId"`
	Metadata struct {
		Reason         string `json:"reason"`
		Answer         string `json:"answer"`
		Question       string `json:"question"`
		ExpectedAnswer string `json:"expected_answer"`
	} `json:"metadata"`
}
