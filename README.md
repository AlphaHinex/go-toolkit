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
```
