Random Pick
===========

随机选择指定类型文件，复制或移动到指定路径。

```bash
$ ./random-pick -h
NAME:
   random-pick - Random pick files in some path

USAGE:
   random-pick [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   -n value    Pick n files (default: 10)
   -t value    File type(s) to pick, * means all types, comma separated for multi values: 'jpg,png', case insensitive (default: "*")
   -i value    Path to pick files (default: ".")
   -o value    Output picked files (default: ".")
   -k          Keep picked files in path (default: false)
   --help, -h  show help (default: false)
```

从 `./foo` 路径选择 5 个 jpg 或 png 类型的文件，复制到 `./bar` 路径：

```bash
$ ./random-pick -i ./foo -n 5 -t jpg,png -o ./bar -k
./random-pick -i ./foo -n 5 -t jpg,png -o ./bar -k
Copy ./foo/21670642460.JPG to ./bar/01670679254.JPG
Copy ./foo/31670642460.JPG to ./bar/11670679254.JPG
Copy ./foo/51670642460.JPG to ./bar/21670679254.JPG
Copy ./foo/11670642460.JPG to ./bar/31670679254.JPG
Copy ./foo/71670642460.PNG to ./bar/41670679254.PNG
```
