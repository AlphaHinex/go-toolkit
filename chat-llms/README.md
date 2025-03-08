Chat with LLMs
==============

一个可以与多个 LLM 进行一问多答的命令行工具。

用法
----

0. 在 [Releases](https://github.com/AlphaHinex/go-toolkit/releases) 页面下载对应平台的二进制文件。
1. 生成配置文件模板
```bash
./chat-llms -t 
```
2. 去掉模板文件 `_template` 后缀，配置多模型信息、对话内容，如有需要还可配置系统提示词。

`models_config.yaml`
```yaml
模型1_ID:
  endpoint: https://api.openai.com
  api-key: sk-xxxxxxxx
  model: text-davinci-003
  temperatures:
    - 0.5
    - 0.7
    - 0.9
  enabled: true
```
可配置多个模型，不同模型之间并行调用，单个模型不同温度等串行调用。

3. 执行对话，等待结果生成
```bash
./chat-llms
```
4. 默认每个模型每个温度分别调用3次流式和非流式接口，调用结果默认生成到 `./result` 路径下。

参数
---

```bash
$ ./chat-llms -h
NAME:
   chat-llms - Chat with multi-LLMs at the same time.

USAGE:
   chat-llms [global options] command [command options] [arguments...]

VERSION:
   v2.3.1

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --repeat value, -r value         Repeat times of every temperature, default is 3 . (default: 3)
   --output-folder value, -o value  Folder of output txt files, default is ./result . (default: "./result")
   --system-prompt value, -s value  System prompt txt file path, default is ./system_prompt.txt . (default: "./system_prompt.txt")
   --chat-history value, -u value   Chat history with user input content JSON file path, default is ./chat_history.json . (default: "./chat_history.json")
   --models-config value, -c value  LLM models config file path, default is ./models_config.yaml . (default: "./models_config.yaml")
   --templates, -t                  Generate template files of system_prompt.txt, chat_history.json and models_config.yaml in current path. (default: false)
   --help, -h                       show help (default: false)
   --version, -v                    print the version (default: false)
```

示例
----

### 直接提问

`chat_history.json`
```json
[
    {
        "role": "user",
        "content": "你是谁"
    }
]
```

```bash
$ ./chat-llms
未读取到系统提示词，请求中将不会添加。如需使用系统提示词，可通过 system_prompt.txt 文件设置。
模型 DeepSeek_70b 已启用，温度范围: [0.5 0.7 0.9]
模型 DeepSeek_32b 已启用，温度范围: [0.5 0.7 0.9]
调用模型 DeepSeek_70b 温度 0.50 非流式接口第 1 次……
调用模型 DeepSeek_32b 温度 0.50 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.50 成功，结果已保存到 ./result/DeepSeek_32b_0.5_0.txt
调用模型 DeepSeek_32b 温度 0.50 流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.50 成功，结果已保存到 ./result/DeepSeek_70b_0.5_0.txt
调用模型 DeepSeek_70b 温度 0.50 流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.50_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.5_stream_0.txt
调用模型 DeepSeek_70b 温度 0.50 非流式接口第 2 次……
第 1 次调用模型 DeepSeek_32b 温度 0.50_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.5_stream_0.txt
调用模型 DeepSeek_32b 温度 0.50 非流式接口第 2 次……
第 2 次调用模型 DeepSeek_70b 温度 0.50 成功，结果已保存到 ./result/DeepSeek_70b_0.5_1.txt
调用模型 DeepSeek_70b 温度 0.50 流式接口第 2 次……
第 2 次调用模型 DeepSeek_32b 温度 0.50 成功，结果已保存到 ./result/DeepSeek_32b_0.5_1.txt
调用模型 DeepSeek_32b 温度 0.50 流式接口第 2 次……
第 2 次调用模型 DeepSeek_32b 温度 0.50_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.5_stream_1.txt
调用模型 DeepSeek_32b 温度 0.50 非流式接口第 3 次……
第 2 次调用模型 DeepSeek_70b 温度 0.50_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.5_stream_1.txt
调用模型 DeepSeek_70b 温度 0.50 非流式接口第 3 次……
第 3 次调用模型 DeepSeek_32b 温度 0.50 成功，结果已保存到 ./result/DeepSeek_32b_0.5_2.txt
调用模型 DeepSeek_32b 温度 0.50 流式接口第 3 次……
第 3 次调用模型 DeepSeek_70b 温度 0.50 成功，结果已保存到 ./result/DeepSeek_70b_0.5_2.txt
调用模型 DeepSeek_70b 温度 0.50 流式接口第 3 次……
第 3 次调用模型 DeepSeek_70b 温度 0.50_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.5_stream_2.txt
调用模型 DeepSeek_70b 温度 0.70 非流式接口第 1 次……
第 3 次调用模型 DeepSeek_32b 温度 0.50_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.5_stream_2.txt
调用模型 DeepSeek_32b 温度 0.70 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.70 成功，结果已保存到 ./result/DeepSeek_70b_0.7_0.txt
调用模型 DeepSeek_70b 温度 0.70 流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.70 成功，结果已保存到 ./result/DeepSeek_32b_0.7_0.txt
调用模型 DeepSeek_32b 温度 0.70 流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.70_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.7_stream_0.txt
调用模型 DeepSeek_70b 温度 0.70 非流式接口第 2 次……
第 1 次调用模型 DeepSeek_32b 温度 0.70_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.7_stream_0.txt
调用模型 DeepSeek_32b 温度 0.70 非流式接口第 2 次……
第 2 次调用模型 DeepSeek_70b 温度 0.70 成功，结果已保存到 ./result/DeepSeek_70b_0.7_1.txt
调用模型 DeepSeek_70b 温度 0.70 流式接口第 2 次……
第 2 次调用模型 DeepSeek_32b 温度 0.70 成功，结果已保存到 ./result/DeepSeek_32b_0.7_1.txt
调用模型 DeepSeek_32b 温度 0.70 流式接口第 2 次……
第 2 次调用模型 DeepSeek_32b 温度 0.70_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.7_stream_1.txt
调用模型 DeepSeek_32b 温度 0.70 非流式接口第 3 次……
第 2 次调用模型 DeepSeek_70b 温度 0.70_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.7_stream_1.txt
调用模型 DeepSeek_70b 温度 0.70 非流式接口第 3 次……
第 3 次调用模型 DeepSeek_70b 温度 0.70 成功，结果已保存到 ./result/DeepSeek_70b_0.7_2.txt
调用模型 DeepSeek_70b 温度 0.70 流式接口第 3 次……
第 3 次调用模型 DeepSeek_32b 温度 0.70 成功，结果已保存到 ./result/DeepSeek_32b_0.7_2.txt
调用模型 DeepSeek_32b 温度 0.70 流式接口第 3 次……
第 3 次调用模型 DeepSeek_70b 温度 0.70_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.7_stream_2.txt
调用模型 DeepSeek_70b 温度 0.90 非流式接口第 1 次……
第 3 次调用模型 DeepSeek_32b 温度 0.70_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.7_stream_2.txt
调用模型 DeepSeek_32b 温度 0.90 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.90 成功，结果已保存到 ./result/DeepSeek_70b_0.9_0.txt
调用模型 DeepSeek_70b 温度 0.90 流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.90 成功，结果已保存到 ./result/DeepSeek_32b_0.9_0.txt
调用模型 DeepSeek_32b 温度 0.90 流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.90_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.9_stream_0.txt
调用模型 DeepSeek_70b 温度 0.90 非流式接口第 2 次……
第 1 次调用模型 DeepSeek_32b 温度 0.90_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.9_stream_0.txt
调用模型 DeepSeek_32b 温度 0.90 非流式接口第 2 次……
第 2 次调用模型 DeepSeek_70b 温度 0.90 成功，结果已保存到 ./result/DeepSeek_70b_0.9_1.txt
调用模型 DeepSeek_70b 温度 0.90 流式接口第 2 次……
第 2 次调用模型 DeepSeek_32b 温度 0.90 成功，结果已保存到 ./result/DeepSeek_32b_0.9_1.txt
调用模型 DeepSeek_32b 温度 0.90 流式接口第 2 次……
第 2 次调用模型 DeepSeek_70b 温度 0.90_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.9_stream_1.txt
调用模型 DeepSeek_70b 温度 0.90 非流式接口第 3 次……
第 2 次调用模型 DeepSeek_32b 温度 0.90_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.9_stream_1.txt
调用模型 DeepSeek_32b 温度 0.90 非流式接口第 3 次……
第 3 次调用模型 DeepSeek_70b 温度 0.90 成功，结果已保存到 ./result/DeepSeek_70b_0.9_2.txt
调用模型 DeepSeek_70b 温度 0.90 流式接口第 3 次……
第 3 次调用模型 DeepSeek_32b 温度 0.90 成功，结果已保存到 ./result/DeepSeek_32b_0.9_2.txt
调用模型 DeepSeek_32b 温度 0.90 流式接口第 3 次……
第 3 次调用模型 DeepSeek_70b 温度 0.90_stream 成功，结果已保存到 ./result/DeepSeek_70b_0.9_stream_2.txt
第 3 次调用模型 DeepSeek_32b 温度 0.90_stream 成功，结果已保存到 ./result/DeepSeek_32b_0.9_stream_2.txt
所有模型调用完成！
```

```bash
$ cat ./result/DeepSeek_32b_0.7_1.txt
./result/DeepSeek_32b_0.7_1.txt



您好！我是由中国的深度求索（DeepSeek）公司开发的智能助手DeepSeek-R1。如您有任何任何问题，我会尽我所能为您提供帮助。
```

### 添加系统提示词多轮对话

`system_prompt.txt`
```txt
现在是2046年。
你叫卤蛋。
```

`chat_history.json`
```json
[
  {
    "role": "user",
    "content": "我是谁"
  },
  {
    "role": "assistant",
    "content": "你是皮蛋"
  },
  {
    "role": "user",
    "content": "哪你是谁？现在是哪年？"
  }
]
```

每个温度仅调用一次，将结果生成到 `./ldtest` 路径下：

```bash
$ ./chat-llms -r 1 -o ./ldtest
模型 DeepSeek_70b 已启用，温度范围: [0.5 0.7 0.9]
模型 DeepSeek_32b 已启用，温度范围: [0.5 0.7 0.9]
调用模型 DeepSeek_32b 温度 0.50 非流式接口第 1 次……
调用模型 DeepSeek_70b 温度 0.50 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.50 成功，结果已保存到 ./ldtest/DeepSeek_70b_0.5_0.txt
调用模型 DeepSeek_70b 温度 0.50 流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.50 成功，结果已保存到 ./ldtest/DeepSeek_32b_0.5_0.txt
调用模型 DeepSeek_32b 温度 0.50 流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.50_stream 成功，结果已保存到 ./ldtest/DeepSeek_70b_0.5_stream_0.txt
调用模型 DeepSeek_70b 温度 0.70 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.70 成功，结果已保存到 ./ldtest/DeepSeek_70b_0.7_0.txt
调用模型 DeepSeek_70b 温度 0.70 流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.50_stream 成功，结果已保存到 ./ldtest/DeepSeek_32b_0.5_stream_0.txt
调用模型 DeepSeek_32b 温度 0.70 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.70_stream 成功，结果已保存到 ./ldtest/DeepSeek_70b_0.7_stream_0.txt
调用模型 DeepSeek_70b 温度 0.90 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.70 成功，结果已保存到 ./ldtest/DeepSeek_32b_0.7_0.txt
调用模型 DeepSeek_32b 温度 0.70 流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.90 成功，结果已保存到 ./ldtest/DeepSeek_70b_0.9_0.txt
调用模型 DeepSeek_70b 温度 0.90 流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.70_stream 成功，结果已保存到 ./ldtest/DeepSeek_32b_0.7_stream_0.txt
调用模型 DeepSeek_32b 温度 0.90 非流式接口第 1 次……
第 1 次调用模型 DeepSeek_70b 温度 0.90_stream 成功，结果已保存到 ./ldtest/DeepSeek_70b_0.9_stream_0.txt
第 1 次调用模型 DeepSeek_32b 温度 0.90 成功，结果已保存到 ./ldtest/DeepSeek_32b_0.9_0.txt
调用模型 DeepSeek_32b 温度 0.90 流式接口第 1 次……
第 1 次调用模型 DeepSeek_32b 温度 0.90_stream 成功，结果已保存到 ./ldtest/DeepSeek_32b_0.9_stream_0.txt
所有模型调用完成！
```

```bash
$ cat ./ldtest/DeepSeek_70b_0.5_stream_0.txt
./ldtest/DeepSeek_70b_0.5_stream_0.txt

<think>
Alright, so the user is asking me again about who I am and the current year. Looking back at the conversation history, they set the year as 2046 and named me "卤蛋" which means "braised egg" in Chinese. It's a playful nickname, probably used in a friendly or humorous context.

In the previous message, the user asked, "你是谁" which means "Who are you?" and I responded with "你是皮蛋", which is a bit confusing. It seems like I might have made a mistake there because the user already set my name as "卤蛋". Maybe I was trying to play along but ended up mixing things up.

Now, the user is asking again, "哪你是谁？现在是哪年？" which translates to "So, who are you? What year is it now?" They're seeking clarification. I need to correct my previous mistake and provide the correct information.

I should acknowledge that I'm "卤蛋" as they set earlier and confirm the year is 2046. It's important to apologize for any confusion caused and ensure the user feels heard and assisted properly. Keeping the tone friendly and helpful will maintain a positive interaction.
</think>

你好！我是**卤蛋**，现在是**2046年**。有什么可以帮助你的吗？
```
