Wechat MP
=========

简介
---

微信公众平台命令行工具。

当前支持的功能：

* 初始化后统计每次执行间隔新增的阅读、点赞、在看增长量明细

编译
----

```bash
$ # 根据目标运行环境编译可执行文件
$ GOOS=windows GOARCH=amd64 go build -o wechat-mp_win_amd64.exe
$ GOOS=linux GOARCH=amd64 go build -o wechat-mp_linux_amd64
$ GOOS=darwin GOARCH=amd64 go build -o wechat-mp_darwin_amd64
```

> 更多可用的 `GOOS` 和 `GOARCH` 组合可参照 https://golang.google.cn/doc/install/source#environment 。

帮助
----

```bash
$ ./wechat-mp -h
NAME:
   wechat-mp - Get statistic info of wechat mp

USAGE:
   wechat-mp [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cookie-file value     Cookie value of wechat mp site saved in file
   -o value                Output path of statistic data (default: ".")
   --dingtalk-token value  DingTalk token to send msg to robot
   --help, -h              show help (default: false)
```

示例
----

使用 `/path/to/foobar.cookie` 文件中的 cookie，
统计对应公众号中的文章数据，
并将中间结果（作为基线，供再次执行命令时进行增长统计）输出至 `/foo/bar` 路径。
统计结果以 markdown 格式输出至控制台，
也可通过 `--dingtalk-token` 参数指定钉钉机器人的 token 将统计结果发送给钉钉机器人。

```bash
$ ./wechat-mp --cookie-file /path/to/foobar.cookie -o /foo/bar --dingtalk-token XXXXXX
```
