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
	"math/rand"
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
    
    请根据问题和标准答案，评估回答的内容与标准答案中内容是否在语义上一致（如缺少标准答案中的某些信息，或存在标准答案中没有的内容等，均视为语义不一致），并给出评估依据。
    以 json 结构返回评估结果，score 代表得分，reason 代表原因。
    无区别 score 为 1，有区别为 0，不确定为 -1。

    ## 返回结构示例

    {"score":"1", "reason":"给出评分依据"}

    <问题>
    {question}
    </问题>

    <标准答案>
    {expectedAnswer}
    </标准答案>

    <回答>
    {answer}
    </回答>
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
  columns:
    # 问题列名
    question: question
    # 标准答案列名
    expected-answer: expected-answer
    # 评价标准（值为 = 时表示回答内容必须与标准答案完全一致，其余值或无此列表示语义一致）
    standard: standard

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
  score-name: llm-evaluator
`, evaluatorPrompt)

var inputCsvTemplate = `question,expected-answer,standard
什么原因会导致邮件被退回？,邮件被退回是指你发送的邮件无法被送达到收件人的邮箱。邮件被退回的原因有很多，包括：\n1. 无效的邮箱地址：你输入的邮箱地址可能无效或者有拼写错误，这会导致邮件无法被送达。\n2. 收件人邮箱已满：如果收件人的邮箱已经满了，邮件就会被退回到发件人的邮箱。\n3. 服务器问题：如果收件人的邮箱服务器出现问题或者宕机，邮件就无法被送达并会被退回到发件人的邮箱。\n4. 垃圾邮件过滤器：如果你的邮件被标记为垃圾邮件或者包含某些触发垃圾邮件过滤器的关键词或短语，邮件就会被屏蔽并退回到发件人的邮箱。\n5. 发件人被屏蔽：如果收件人已经屏蔽了你的邮箱地址，你的邮件就会被退回。\n\n为了避免邮件被退回，重要的是要确保邮箱地址正确，收件人的邮箱没有满，以及你的邮件没有被标记为垃圾邮件。你也可以与收件人核实是否收到了邮件，以及他们的邮箱服务器或垃圾邮件过滤器是否有问题。,=
给我写一个垂直轮播的代码，用于我的落地页。,"很抱歉，作为一个AI语言模型，我无法访问您的落地页代码，也不知道您使用的编程语言。但是，我可以给您提供使用HTML、CSS和JavaScript创建垂直轮播的一般思路。\n\n以下是一个示例：\n\nHTML：\n` + "```" + `\nhtml<div class=""carousel"">\n  <div class=""carousel-inner"">\n    <div class=""carousel-item active"">\n      <img src=""image1.jpg"" alt=""Image 1"">\n    </div>\n    <div class=""carousel-item"">\n      <img src=""image2.jpg"" alt=""Image 2"">\n    </div>\n    <div class=""carousel-item"">\n      <img src=""image3.jpg"" alt=""Image 3"">\n    </div>\n  </div>\n</div>\n` + "```" + `\nCSS：\n` + "```" + `css\n.carousel {\n  position: relative;\n  width: 100%;\n  height: 500px; /* 设置轮播的高度 */\n  overflow: hidden;\n}\n.carousel-inner {\n  position: absolute;\n  top: 0;\n  left: 0;\n  height: 100%;\n  width: 100%;\n  display: flex;\n  flex-direction: column;\n  transition: transform 0.6s ease-in-out;\n}\n.carousel-item {\n  flex: 1 0 auto;\n  position: relative;\n  display: flex;\n  justify-content: center;\n  align-items: center;\n}\n.carousel-item img {\n  height: 100%;\n  width: 100%;\n  object-fit: cover;\n}\n` + "```" + `\nJavaScript：\n` + "```" + `javascript\nlet slideIndex = 1;\nshowSlides(slideIndex);\nfunction plusSlides(n) {\n  showSlides(slideIndex += n);\n}\nfunction showSlides(n) {\n  let i;\n  let slides = document.getElementsByClassName(""carousel-item"");\n  if (n > slides.length) {slideIndex = 1}\n  if (n < 1) {slideIndex = slides.length}\n  for (i = 0; i < slides.length; i++) {\n      slides[i].style.transform = ""translateY("" + ((i-slideIndex+1)*100) + ""%)"";\n  }\n}\n` + "```" + `\n在HTML中，具有“carousel”类的“div”是轮播的容器。在此容器内部是另一个具有“carousel-inner”类的“div”，它将包含单个轮播项，每个项都包装在具有“carousel-item”类的“div”中。CSS设置轮播的高度，隐藏任何溢出，并使用flexbox垂直排列项。JavaScript将初始幻灯片索引设置为1，并定义“showSlides”函数，该函数接受一个参数“n”，表示要显示的幻灯片。 “plusSlides”函数通过参数“n”更新幻灯片索引并调用“showSlides”。在“showSlides”中，当前幻灯片垂直翻译到轮播容器的顶部，而其他幻灯片则翻译到其下方。\n\n这只是创建垂直轮播的一种示例。您可能需要修改代码以适应您特定的用例和偏好。",%
"[
    {
        ""role"": ""user"",
        ""content"": ""大多数人在网上搜索什么类型的地板？""
    },
    {
        ""role"": ""user"",
        ""content"": ""在英国，大多数人在网上搜索什么类型的地板？""
    }
]","[
    {
        ""role"": ""user"",
        ""content"": ""大多数人在网上搜索什么类型的地板？""
    },
    {
        ""role"": ""assistant"",
        ""content"": ""当涉及到地板时，很难说大多数人在网上搜索什么类型的地板，因为这取决于地理位置、个人喜好和当前的趋势。一些经常在网上搜索的受欢迎的地板类型包括实木、强化木地板、瓷砖、地毯和乙烯基。""
    },
    {
        ""role"": ""user"",
        ""content"": ""在英国，大多数人在网上搜索什么类型的地板？""
    },
    {
        ""role"": ""assistant"",
        ""content"": ""很难准确地说英国大多数人在网上搜索什么类型的地板，因为这取决于地理位置、个人喜好和当前的趋势。然而，在英国，一些经常在网上搜索的受欢迎的地板类型包括：\n* 地毯\n* 强化木地板\n* 实木\n* 乙烯基\n* 瓷砖（如陶瓷或瓷砖）\n* 工程木\n* 豪华乙烯基瓷砖（LVT）\n* 天然石材\n需要注意的是，某些类型的地板的受欢迎程度会随着时间的推移而改变，因此最好定期检查搜索数据和趋势以保持最新。""
    }
]",%
"[
    {
        ""role"": ""user"",
        ""content"": ""AI中的N-Tile问题？""
    },
    {
        ""role"": ""user"",
        ""content"": ""上述的启发式算法""
    }
]","[
    {
        ""role"": ""user"",
        ""content"": ""AI中的N-Tile问题？""
    },
    {
        ""role"": ""assistant"",
        ""content"": ""AI中的N-Tile问题是将一组项目分成N个子集，使得每个子集尽可能相似的问题。该问题通常用于机器学习中将数据集分成训练、验证和测试集，或将人口分组进行研究。该问题可以使用各种优化算法解决，例如线性规划或遗传算法。要优化的目标函数通常是子集的方差或熵的总和。""
    },
    {
        ""role"": ""user"",
        ""content"": ""上述的启发式算法""
    },
    {
        ""role"": ""assistant"",
        ""content"": ""有几种启发式算法可用于解决N-Tile问题。其中一些最常见的包括：\n1. 随机抽样：这涉及从集合中随机选择项目并将其放入子集中。这是一种简单快速的方法，但可能不会导致最优解。\n2. K-means聚类：这涉及根据其特征对项目进行聚类，然后将聚类分成子集。\n3. 遗传算法：这涉及使用遗传算法演化解决方案的种群，其中每个解决方案表示将项目划分为子集的可能性。\n4. 贪心算法：这涉及从空子集开始，反复添加最大程度减少子集方差的下一个项目。\n5. 局部搜索：这涉及从初始随机解开始，反复对子集进行小的更改，以尝试改善目标函数。\n6. 分而治之：这涉及将项目集递归地划分为越来越小的子集，直到每个子集仅包含一个项目。\n启发式算法的选择取决于特定的用例和可用资源。""
    }
]",%`

var outputFolder = ""
var parallel = 0
var prefix = ""
var samplingRate = 1.0
var debugEnabled = false

func main() {
	app := &cli.App{
		Name:    "llm-evaluator",
		Usage:   "Evaluate QA capability of LLM model with LLM model.",
		Version: "v2.5.0",
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
				Usage:    "Generate template files of configs.yaml (and input csv example) in current path.",
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
			&cli.Float64Flag{
				Name:     "sampling-rate",
				Aliases:  []string{"r"},
				Usage:    "Sampling rate of all questions. 0.8 means 80% of questions will be sampled for evaluation. 1.0 means all questions will be evaluated",
				Value:    1.0,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "debug",
				Aliases:  []string{"d"},
				Usage:    "Enable debug mode, which will write more debug info into evaluation result file. Default is false.",
				Value:    false,
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {
			start := time.Now()
			needTemplates := cCtx.Bool("templates")
			configsFilePath := cCtx.String("configs")
			parallel = cCtx.Int("parallel")
			samplingRate = cCtx.Float64("sampling-rate")
			debugEnabled = cCtx.Bool("debug")

			if needTemplates {
				err := os.WriteFile("configs.yaml_template", []byte(strings.TrimSpace(configsTemplate)), 0644)
				if err != nil {
					log.Fatalf("生成配置文件模板失败: %v", err)
				} else {
					log.Println("生成配置文件模板成功！")
				}
				err = os.WriteFile("input.csv_template", []byte(strings.TrimSpace(inputCsvTemplate)), 0644)
				if err != nil {
					log.Fatalf("生成 CSV 文件模板失败: %v", err)
				} else {
					log.Println("生成 CSV 文件模板成功！")
				}
			} else {
				configs := readConfigs(configsFilePath)
				outputFolder = configs.Output.Folder
				if strings.TrimSpace(outputFolder) == "" {
					// 使用默认输出路径
					outputFolder = "./result"
				}
				prefix = fmt.Sprintf("%s/%s_%s_", outputFolder, configs.Model.Candidate.Model, time.Now().Format("20060102_150405"))
				backupConfigsToOutputFolder(configs, outputFolder)

				doEvaluate(configs)
			}
			log.Printf("完成评估，总耗时: %s\n", time.Since(start))
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
		log.Panicf("读取输入文件失败: %v", err)
	}
	// 从首行查找问题、标准答案、评价标准列索引
	qIndex, aIndex, sIndex := -1, -1, -1
	for i, header := range qa[0] {
		if header == configs.Input.Columns.Question {
			qIndex = i
		} else if header == configs.Input.Columns.ExpectedAnswer {
			aIndex = i
		} else if header == configs.Input.Columns.Standard {
			sIndex = i
		}
	}
	if qIndex < 0 || aIndex < 0 {
		log.Panicf("输入文件格式错误，必须包含 %s 和 %s 列", configs.Input.Columns.Question, configs.Input.Columns.ExpectedAnswer)
	}

	candidateModel := configs.Model.Candidate
	evaluatorModel := configs.Model.Evaluator

	// 定义存放评估结果的 channel
	results := make(chan string)
	// 定义一个带缓冲的 channel 作为信号量
	semaphore := make(chan struct{}, parallel) // parallel 是并发限制数量

	// 如果采样率小于 1.0，则随机采样
	if samplingRate < 1.0 {
		sampledQA := [][]string{qa[0]} // 保留表头
		for _, record := range qa[1:] {
			if len(sampledQA) == 1 || (samplingRate > 0 && rand.Float64() < samplingRate) {
				sampledQA = append(sampledQA, record)
			}
		}
		qa = sampledQA
		log.Printf("采样后问题数量: %d\n", len(qa)-1)
	} else {
		log.Printf("未进行采样，问题数量: %d\n", len(qa)-1)
	}

	var wg sync.WaitGroup
	for _, record := range qa[1:] { // 跳过表头
		wg.Add(1)
		go func(oneLine []string) {
			defer wg.Done()

			// 占用一个并发槽
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // 释放并发槽

			// 获取问题和标准答案
			question := oneLine[qIndex]
			if len(strings.TrimSpace(question)) == 0 {
				return
			}
			expectedAnswer := oneLine[aIndex]

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
			var id, answer, debugInfo string
			var duration, totalDuration, evaluationDuration int64
			var chatHistory []Message
			for _, q := range questions {
				// 调用候选模型作答
				id, answer, duration, err = callChatAPI(candidateModel, true, q, chatHistory)
				debugInfo += fmt.Sprintf("[%s] %d (%v) ", id, duration, err)
				totalDuration += duration
				if err != nil {
					log.Printf("%s 候选模型调用失败: %v\n", candidateModel.Model, err)
					continue
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
					log.Printf("序列化聊天记录失败: %v\n", err)
					return
				}
				answer = string(chatHistoryJSON)
			}

			score := ""
			reason := ""
			if sIndex > 0 && oneLine[sIndex] == "=" {
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
				evaId, scoreWithReason, duration, err := callChatAPI(evaluatorModel, true, getEvaluatePrompt(configs.Prompt.Evaluator, question, answer, expectedAnswer), nil)
				debugInfo += fmt.Sprintf("[%s] %d (%v) ", evaId, duration, err)
				evaluationDuration = duration
				if err != nil {
					log.Printf("%s 评估模型调用失败: %v\n", evaluatorModel.Model, err)
					return
				}
				scoreWithReason = cleanThinkOfDeepSeek(scoreWithReason)
				scoreWithReason = cleanMarkdownJsonSymbolIfNeeded(scoreWithReason)
				// 将 scoreWithReason 转成 json
				var result map[string]interface{}
				err = json.Unmarshal([]byte(scoreWithReason), &result)
				if err != nil {
					log.Printf("解析评估结果失败！\n%s%v\n", scoreWithReason, err)
					debugInfo += fmt.Sprintf("解析 JSON 字符串 %s 失败 (%v) ", scoreWithReason, err)
				} else {
					score = fmt.Sprint(result["score"])
					reason = fmt.Sprint(result["reason"])
				}
			}

			log.Printf("[DEBUG] %s", debugInfo)
			if debugEnabled {
				// debug 模式增加三列输出：问答耗时、评估耗时、debug 信息
				debugInfo = fmt.Sprintf(",%d,%d,%s", totalDuration, evaluationDuration, toOneCell(debugInfo))
			} else {
				debugInfo = ""
			}
			results <- fmt.Sprintf("%s,%s,%s,%s%s", strings.Join(toOneCells(oneLine), ","), toOneCell(answer), toOneCell(score), toOneCell(reason), debugInfo)
			if configs.Langfuse.Enable {
				wg.Add(1)
				go func() {
					defer wg.Done()
					createLangfuseScore(configs, id, score, question, answer, expectedAnswer, reason)
				}()
			}
		}(record)
	}

	// 启动一个 goroutine 关闭 channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// 将 channel 中的内容写入文件
	outputFilePath := fmt.Sprintf("%s_evaluate_result.csv", prefix)
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		log.Fatalf("创建输出文件失败: %v", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	debugTitle := ""
	if debugEnabled {
		debugTitle = ",answer_duration(ms),evaluation_duration(ms),debug_info"
	}
	_, err = writer.WriteString(fmt.Sprintf("%s,answer,score,reason%s\n", strings.Join(qa[0], ","), debugTitle))
	if err != nil {
		log.Fatalf("写入文件失败: %v", err)
	}
	for result := range results {
		_, err = writer.WriteString(result + "\n")
		if err != nil {
			log.Fatalf("写入文件失败: %v", err)
		}
	}
	err = writer.Flush()
	if err != nil {
		log.Fatalf("刷新输出文件失败: %v", err)
	}
	log.Printf("结果已写入文件: %s\n", outputFilePath)
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

func callChatAPI(model ModelConfig, isStream bool, userPrompt string, history []Message) (string, string, int64, error) {
	log.Printf("调用模型 %s %s，温度 %.2f，流式: %t\n", model.Endpoint, model.Model, model.Temperature, isStream)
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
		log.Fatalf("序列化请求体失败: %v", err)
	}

	client := &http.Client{
		Timeout: 600 * time.Second, // 设置超时时间为 600 秒
	}
	req, err := http.NewRequest("POST", model.Endpoint+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		log.Fatalf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+model.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()                                                      // 记录开始时间
	resp, err := doRequestWithRetry(req, client, jsonBody, 3, 2*time.Second) // 重试 3 次，每次重试等待递增间隔 2 秒
	if err != nil {
		log.Fatalf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", "", time.Since(start).Milliseconds(), fmt.Errorf("API返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
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
					log.Fatalf("解析响应失败: %v", err)
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
			log.Fatalf("解析响应失败: %v", err)
		}
		if len(apiResp.Choices) > 0 {
			content = apiResp.Choices[0].Message.Content
			id = apiResp.Id
		}
	}
	duration := time.Since(start) // 计算调用时长
	log.Printf("\n%s model input (%s):\n%s\n", model.Model, id, userPrompt)
	log.Printf("\n%s model output (%s):\n%s\n", model.Model, id, content)
	log.Printf("\n%s model (%s) call duration %v (%s start) \n", model.Model, id, duration, start)
	return id, content, duration.Milliseconds(), nil
}

// doRequestWithRetry 执行 HTTP 请求并支持重试机制
func doRequestWithRetry(req *http.Request, client *http.Client, requestBody []byte, maxRetries int, retryDelay time.Duration) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= maxRetries; i++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			if i > 0 {
				log.Printf("%s 请求第 %d 次成功.\n请求体：\n%s\n", req.URL, i+1, requestBody)
			}
			return resp, nil
		}

		// 如果不是最后一次重试，等待一段时间后重试
		if i < maxRetries {
			log.Printf("%s 请求失败，稍后第 %d 次重试...\n请求体：\n%s\n错误信息：\n%v\n", req.URL, i+1, requestBody, err)
			time.Sleep(time.Duration(i) * retryDelay)
			req.Body = io.NopCloser(bytes.NewReader(requestBody)) // 重置请求体
		}
	}

	// 返回最后一次的响应或错误
	return resp, err
}

func cleanThinkOfDeepSeek(content string) string {
	// 定义多行匹配的正则表达式
	// (?s) 启用单行模式，使 . 可以匹配换行符
	re := regexp.MustCompile(`(?s)(<think>)?.*?</think>`)

	// 替换匹配的内容
	return strings.TrimSpace(re.ReplaceAllString(content, ""))
}

func cleanMarkdownJsonSymbolIfNeeded(content string) string {
	startIdx := strings.Index(content, "```json")
	endIdx := strings.LastIndex(content, "```")
	if startIdx > -1 && endIdx > -1 {
		content = content[startIdx+7 : endIdx]
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
	fileName := fmt.Sprintf("%s_configs.yaml", prefix)
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
		log.Panicf("打开输入文件失败: %v", err)
	}
	defer file.Close()

	// 创建 CSV 读取器
	reader := csv.NewReader(file)
	// 读取所有行
	records, err := reader.ReadAll()
	if err != nil {
		log.Panicf("读取输入文件失败: %v\n", err)
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
		log.Printf("序列化 Langfuse Score 请求体异常 %s", err)
	}

	req, err := http.NewRequest("POST", configs.Langfuse.Host+"/api/public/scores", bytes.NewReader(jsonBody))
	req.SetBasicAuth(configs.Langfuse.PublicKey, configs.Langfuse.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := doRequestWithRetry(req, client, jsonBody, 3, 2*time.Second) // 重试 3 次，每次重试等待递增间隔 2 秒
	if err != nil {
		log.Printf("Langfuse 创建 Score 请求异常，请求体：\n %s\n异常信息：%v", jsonBody, err)
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	if statusCode != 200 {
		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("读取创建 Langfuse Score 请求响应体异常 %v", err)
		}
		log.Printf("调用 Langfuse 失败：%s %s\n", resp.Status, string(body))
	}
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
		File    string `yaml:"file"`
		Columns struct {
			Question       string `yaml:"question"`
			ExpectedAnswer string `yaml:"expected-answer"`
			Standard       string `yaml:"standard"`
		} `yaml:"columns"`
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
