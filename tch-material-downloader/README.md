tch-material-downloader
=======================

下载国家中小学智慧教育平台（ https://basic.smartedu.cn/tchMaterial ）教材资源：
- 指定课程 ID 后，自动抓取全部音频并下载到本地。
- 自动从详情 JSON 中识别并下载课本 PDF。
- 输出 `manifest.json` 记录每个文件下载结果。
- 若课程无音频或音频清单不可用，会自动跳过音频，仅下载 PDF。

前置条件
--------

注册国家中小学智慧教育平台账号并登录，选定某个教材打开页面，如初中英语北师大版七年级上，
对应链接为：https://basic.smartedu.cn/tchMaterial/detail?contentType=assets_document&contentId=331959b8-d722-eac6-5466-4bac34806136&catalogType=tchMaterial&subCatalog=tchMaterial

![](https://alphahinex.github.io/contents/tch-material-downloader/tch-material.png)

### 获取 MAC id

打开浏览器开发者工具（一般按 F12），切换到 Network（网络）选项卡，刷新页面，找到包含 `pdf` 的 `GET` 请求，查看请求头 `X-Nd-Auth` 中包含的 MAC id 值：

![](https://alphahinex.github.io/contents/tch-material-downloader/ids.png)

### 获取课程 ID

从教材页面 URL 中的 `contentId=xxx` 获得课程 ID，如上面链接中 `contentId=331959b8-d722-eac6-5466-4bac34806136`，则课程 ID 为 `331959b8-d722-eac6-5466-4bac34806136`。

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
  --course-id '<course-id>' \
  --output ./downloads

# 或直接运行后按提示输入 mac-id 和 course-id
./tch-material-downloader
```

常用参数：

- `--mac-id`, `-m`: 下载鉴权头 `x-nd-auth` 的值（必填）
- `--course-id`, `-c`: 课程 ID（必填）
- `--output`, `-o`: 输出目录（可选，默认 `./output/{course-name}`）
- `--timeout`: 请求超时秒数，默认 45

输出说明
--------

下载完成后，输出目录下会有：

- 按顺序命名的音频文件：`001_*.mp3`, `002_*.mp3`, ...
- 课本 PDF
- `manifest.json`：包含成功/失败、源 URL、保存路径、大小等信息
