GitLab Analyzer
===============

调用 GitLab RESTful API，分析指定项目和分支在某时间范围内的 Commit 情况，对每个 commit 中修改的文件进行逐个分析，统计新增代码行数、减少代码行数，以及忽略空格和换行改动的新增代码行数、减少代码行数（相当于 `git diff -w`），将分析结果生成 csv 文件，并按提交人邮箱进行汇总排名，输出至 console，并可通过飞书机器人发送统计结果。支持过滤初始 Commit 及 Merge Request Commit。

主要使用了以下两个 API：

1. [/help/api/projects.md](https://docs.gitlab.com/ee/api/projects.html)
1. [/help/api/commits.md](https://docs.gitlab.com/ee/api/commits.html)

```bash
$ ./gitlab -h
NAME:
   gitlab - Use GitLab API to do some analysis works

USAGE:
   gitlab [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --url value, -u value            GitLab host url
   --access-token value, -t value   Access token to use GitLab API
   --project-id value, -p value     Project ID in GitLab
   --branch value, -b value         Branch of project
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
```

统计 https://gitlab.com/gnachman/iterm2 项目 2022 年 11 月代码提交情况：

```bash
$ ./gitlab -u https://gitlab.com/ -t XXXXXX -p 252461 -b master --commit-parents 1 --since 2022-11-01 --until 2022-11-30
2022/12/10 22:47:19 Start to analyse iterm2 ...
2022/12/10 22:47:22 Load all commits
2022/12/10 22:47:31 Generate 252461_iterm2_master_2022-11-01~2022-11-30.csv use 24.443924546s.

iterm2 项目 master  分支代码分析结果（2022-11-01~2022-11-30)

No. author                    effLines(ratio)	effAdds(ratio)	commits	files
 1. gnachman@gmail.com        2366(90.31%)	1538(90.26%)	23	64
 2. brewingcode@users.noreply.github.com 2(50.00%)	2(50.00%)	1	2

以上结果统计了除初始 Commit 和 Merge Request 外的所有 Commit（时间范围内）
* effLines（有效代码行数）= 有效增加代码行数 + 有效减少代码行数
* effLines ratio（有效代码率）= 有效代码行数 / 总代码行数 * 100%
* effAdds（有效增加行数）= 有效增加代码行数
* effAdds ratio（有效增加率）= 有效增加代码行数 / 总增加代码行数 * 100%
* commits：Commit 总数
* files：文件总数（不去重）
* 有效代码：忽略仅有空格或换行的代码改动，diff -w
```
