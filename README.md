README
======

Base on https://cli.urfave.org/

How to build
------------

Enter each folder to execute build command:

```bash
$ cd random-pick
# 编译为可在当前环境运行的可执行文件
$ go build
# 编译为可在其他环境运行的可执行文件
$ GOOS=windows GOARCH=amd64 go build
$ GOOS=linux GOARCH=amd64 go build
# -o 可设置编译出的可执行文件名称
$ GOOS=windows GOARCH=amd64 go build -o test_win_amd64.exe
```

> 更多可用的 GOOS 和 GOARCH 组合可参照 https://golang.google.cn/doc/install/source#environment 。

Template
--------

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "sonar-exp",
		Usage:   "Export sonar projects info into csv",
		Version: "v2.6.2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "host",
				Usage:    "Sonar host",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "token",
				Aliases:  []string{"t"},
				Usage:    "User token",
				Required: true,
			},
		},
		Action: func(cCtx *cli.Context) error {
			host := cCtx.String("host")
			token := cCtx.String("token")
			fmt.Printf("boom! I say! %s, %s", host, token)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
```