GitLab Analyzer
===============

[GitLab Analyser](https://github.com/AlphaHinex/go-toolkit/tree/main/gitlab) 是一个使用 [Golang](https://go.dev/) 编写的跨平台命令行工具。

通过调用 [GitLab REST API](https://docs.gitlab.com/ee/api/rest/) ，可分析指定项目和分支在某时间范围内的 Commit 情况，包括：
1. 统计每个提交中修改的所有文件
1. 统计新增代码行数、减少代码行数 —— 相当于 `git diff`
1. 统计有效新增代码行数（忽略空格和换行的新增代码行数）、有效减少代码行数 —— 相当于 `git diff -w`

统计结果按提交人邮箱进行汇总后，按有效代码总行数排名，并输出至 console。

同时，将所有提交的分析明细数据输出至命令执行路径下 CSV 文件中，还可通过指定 [飞书机器人](https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN) 的 webhook 地址发送统计结果。

主要使用了以下两个 API：

1. [/help/api/projects.md](https://docs.gitlab.com/ee/api/projects.html)
1. [/help/api/commits.md](https://docs.gitlab.com/ee/api/commits.html)

## 使用示例

GitLab 地址及 Project ID 为必须指定的参数，其余参数均为可选。
* GitLab 地址为访问 GitLab 仓库的根路径，如：https://gitlab.com/、http://192.168.16.24:8888/ 。
* Project ID 可从项目首页获得，如 https://gitlab.com/gnachman/iterm2 项目的 ID 为 `252461` 。
  ![](https://alphahinex.github.io/contents/gitlab-cli/iterm2.png)

### 查看版本及帮助信息

```bash
$ ./gitlab -h
   gitlab - Use GitLab API to analyse commits

USAGE:
   gitlab [global options] command [command options] [arguments...]

VERSION:
   v2.1.1

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --url value, -u value            GitLab host url, required, like https://gitlab.com/
   --access-token value, -t value   Access token to use GitLab API
   --project-ids value, -p value    Project IDs in GitLab, required, could multi: 5,7-10,13-25
   --branch value, -b value         Branch of project, will analyse all branches if not set
   --since value                    Date of since, from 00:00:00 (default: "2022-01-01")
   --until value                    Date of until, to 23:59:59 (default: "2022-12-31")
   --parallel value                 Number of commit parsers (default: 16)
   --lark value                     Lark webhook url
   --commit-parents commit-parents  Only count the commit has commit-parents number parent(s), 
                                        -1 means counting all commits, 
                                        0 means only counting the initial commit, 
                                        2 means only counting merge request commit, 
                                        1 means exclude initial commit and merge request commit (default: -1)
   --help, -h                       show help (default: false)
   --version, -v                    print the version (default: false)
```

### 指定项目、分支、时间范围

分析 `iterm2` 项目 `master` 分支 2018 年 7 月代码提交情况：

```bash
$ ./gitlab -u https://gitlab.com -p 252461 -b master --since 2018-07-01 --until 2018-07-31
2023/04/29 11:33:35 Start to analyse master branch of iterm2 project ...
2023/04/29 11:33:37 Load all commits
2023/04/29 11:33:45 Generate 252461_iterm2_master_2018-07-01~2018-07-31.csv use 9.814436582s.

iterm2 项目 master  分支代码分析结果（2018-07-01~2018-07-31)

No. author                                             effLines(ratio)	effAdds(ratio)	commits	files
 1. George Nachman(gnachman@gmail.com)                 25053(85.28%)	20006(85.44%)	143	886
 2. George Nachman(gln@whatsapp.com)                   2050(82.93%)	1314(82.49%)	29	88
 3. Stefan Sundin(git@stefansundin.com)                12(92.31%)	10(90.91%)	3	5
 4. George Nachman(gnachman+github@gmail.com)          10(90.91%)	9(90.00%)	1	3

以上结果统计了 Parent 数量为 -1 的 Commit（时间范围内）
* effLines（有效代码行数）= 有效增加代码行数 + 有效减少代码行数
* effLines ratio（有效代码率）= 有效代码行数 / 总代码行数 * 100%
* effAdds（有效增加行数）= 有效增加代码行数
* effAdds ratio（有效增加率）= 有效增加代码行数 / 总增加代码行数 * 100%
* commits：Commit 总数
* files：文件总数（不去重）
* 有效代码：忽略仅有空格或换行的代码改动，diff -w
```

### 忽略初始 Commit 及 Merge Request Commit

`--commit-parents 1` 排除初始提交和 Merge Request 提交。

设为 `0` 时仅统计初始化提交，设为 `2` 时仅统计 Merge Request 提交。

```bash
$ ./gitlab -u https://gitlab.com -p 252461 -b master --since 2018-07-01 --until 2018-07-31 --commit-parents 1
2023/04/29 11:33:56 Start to analyse master branch of iterm2 project ...
2023/04/29 11:33:58 Load all commits
2023/04/29 11:34:07 Generate 252461_iterm2_master_2018-07-01~2018-07-31.csv use 10.985404116s.

iterm2 项目 master  分支代码分析结果（2018-07-01~2018-07-31)

No. author                                             effLines(ratio)	effAdds(ratio)	commits	files
 1. George Nachman(gnachman@gmail.com)                 24824(85.18%)	19861(85.37%)	142	878
 2. George Nachman(gln@whatsapp.com)                   2050(82.93%)	1314(82.49%)	29	88
 3. Stefan Sundin(git@stefansundin.com)                12(92.31%)	10(90.91%)	3	5

以上结果统计了除初始 Commit 和 Merge Request 外的所有 Commit（时间范围内）
* effLines（有效代码行数）= 有效增加代码行数 + 有效减少代码行数
* effLines ratio（有效代码率）= 有效代码行数 / 总代码行数 * 100%
* effAdds（有效增加行数）= 有效增加代码行数
* effAdds ratio（有效增加率）= 有效增加代码行数 / 总增加代码行数 * 100%
* commits：Commit 总数
* files：文件总数（不去重）
* 有效代码：忽略仅有空格或换行的代码改动，diff -w
```

### 分析所有分支

不指定分支参数时，可分析所有分支：

```bash
$ ./gitlab -p 252461 -u https://gitlab.com/ --since 2022-12-24 --until 2023-04-28
```

### 分析多个项目

可使用逗号间隔多个项目 ID，`-` 表示连续的 ID 范围，如

```bash
$ ./gitlab -u https://gitlab.com -p 5,7-10,13-25,252461 -b master --since 2018-07-01 --until 2018-07-29
```

### 指定 access token

[/-/profile/personal_access_tokens](https://gitlab.com/-/profile/personal_access_tokens) 界面生成 Access Token 后，通过 `-t` 参数传入，即可访问私有仓库，如：

```bash
$ ./gitlab -u https://gitlab.com -t XXXXXX -p 6777 -b develop --commit-parents 1
```

### 发送飞书通知

获得飞书自定义机器人的 webhook 地址后，通过 `--lark` 参数传入，即可在分析结束后，将控制台中输出的统计信息，通过飞书机器人发送至飞书群中：

```bash
$ ./gitlab -u https://gitlab.com -p 252461 -b master --since 2018-07-01 --until 2018-07-29 --lark https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxxxxxxxxxxx
```

![](https://alphahinex.github.io/contents/gitlab-cli/lark.png)

## 合并统计结果

分析得到多个明细 CSV 文件时，可以使用如下脚本合并为一个 CSV 以便后续进行使用：

```bash
$ cat merge.sh
echo "project\tbranch\tsha\tdate\tauthor\temail\tfilename\tfiletype\toperation\tadd\tdel\taddIgnoreSpace\tdelIgnoreSpace" > merge.csv
cat *_*-*~*-*.csv | grep -v "project\tbranch\tsha\tdate\tauthor\temail\tfilename\tfiletype\toperation\tadd\tdel\taddIgnoreSpace\tdelIgnoreSpace" >> merge.csv
```