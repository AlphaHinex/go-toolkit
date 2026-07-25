tch-material-downloader
=======================

下载国家中小学智慧教育平台教材资源：
- 指定课程 ID 后，自动抓取全部音频并下载到本地。
- 自动从详情 JSON 中识别并下载课本 PDF。
- 输出 `manifest.json` 记录每个文件下载结果。
- 若课程无音频或音频清单不可用，会自动跳过音频，仅下载 PDF。

前置条件
--------

1. 你需要一个可用的 `x-nd-auth` 值（本工具中通过 `--mac-id` 传入）。
2. 该值通常来自已登录浏览器请求头，失效后需要重新获取。

构建
----

```bash
go build
```

用法
----

```bash
./tch-material-downloader \
  --mac-id '<x-nd-auth-value>' \
  --course-id 'db7a6bc2-c5d4-4115-88e5-bf6f36cf1914' \
  --output ./downloads

# 或直接运行后按提示输入 mac-id 和 course-id
./tch-material-downloader
```

常用参数：

- `--mac-id`, `-m`: 下载鉴权头 `x-nd-auth` 的值（必填）
- `--course-id`, `-c`: 课程 ID（必填）
- `--output`, `-o`: 输出目录（可选，默认 `./output/{course-id}`）
- `--timeout`: 请求超时秒数，默认 45
- `--referer`: 请求 Referer，默认 `https://basic.smartedu.cn/`
- `--ua`: 请求 User-Agent，可按需覆盖

输出说明
--------

下载完成后，输出目录下会有：

- 按顺序命名的音频文件：`001_*.mp3`, `002_*.mp3`, ...
- 课本 PDF：`textbook_01_*.pdf`（如有多个会继续递增）
- `manifest.json`：包含成功/失败、源 URL、保存路径、大小等信息
