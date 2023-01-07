mdnice
======

上传图片至 [mdnice](https://editor.mdnice.com/) 图床的命令行工具。

```bash
$ ./mdnice -h
NAME:
   mdnice - Upload pictures to mdnice

USAGE:
   mdnice [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   -i value                 Path to be uploaded (default: ".")
   --token value            Bearer token of mdnice
   --token-file value       Bearer token file of mdnice
   --img-path-prefix value  Path to add before image link (local file path) in markdown file
   --help, -h               show help (default: false)
```

批量上传图片
----------

将指定路径下的所有图片文件，上传至图床，需要 mdnice 的 JWT（JSON Web Token）。图片上传到图床后的链接以 markdown 格式输出到图片来源路径的 README.md 文件中，上传失败的也会将失败原因记录至该 md 文件。

> 如何获取 JWT？
> 
> 浏览器访问 https://editor.mdnice.com/ ，登录后，打开开发者工具进行网络监控，刷新页面，选择 `Fetch/XHR` 类请求中 Request Headers 带 `Authorization` 的请求，其值即为 JWT，可通过参数传入，或保存至文件。
> 
> 注意：传入的 token 需包含前面的 `Bearer `。

![](https://files.mdnice.com/user/30377/6755803f-41cd-48d6-9076-f0c93ec4b0c3.jpg)

### 示例

使用 token 文件中的 JWT，将 `./foo` 路径下的所有（图片）文件上传：

```bash
$ ./mdnice -i ./foo --token-file token
Failed to upload 01670642460.GIF
Upload 01670642460.JPG done
Upload 11670642460.JPG done
Upload 21670642460.JPG done
Upload 31670642460.JPG done
Upload 41670642460.JPG done
Upload 51670642460.JPG done
Upload 61670642460.JPG done
Failed to upload 71670642460.PNG
Upload 81670642460.JPG done
Failed to upload README.md
$ cat ./foo/README.md
![](https://files.mdnice.com/user/30377/89e8cb29-4f58-4afc-a9cd-37018de437e3.JPG)
![](https://files.mdnice.com/user/30377/263b7008-eb99-4a0a-b502-2b3b1ceb6e3c.JPG)
![](https://files.mdnice.com/user/30377/4df30bf5-b763-4c94-801b-a7f52573e5c1.JPG)
![](https://files.mdnice.com/user/30377/bf4099ce-b81e-4320-aed7-7501bf06a22f.JPG)
![](https://files.mdnice.com/user/30377/f6183597-1929-42c9-bfa8-962b9521b0c7.JPG)
![](https://files.mdnice.com/user/30377/6f204840-cedd-4d67-909f-ef373bdf5443.JPG)
![](https://files.mdnice.com/user/30377/d42fa98a-0f30-4a87-8a26-32add001aa8d.JPG)
![](https://files.mdnice.com/user/30377/1a5152b1-a665-461e-8324-b58e3209a13a.JPG)

---
1. Upload ./foo/01670642460.GIF failed: 50005:文件过大
1. Upload ./foo/71670642460.PNG failed: 50005:文件过大
1. Upload ./foo/README.md failed: 50005:文件类型错误，仅支持jpg、jpeg、png、gif、svg类型
```

替换 markdown 中的本地图片
-----------------------

markdown 文档引入本地图片文件时，可通过此工具将文档中的本地图片上传至 mdnice 图床，并将图片链接替换为 mdnice 图床的链接。替换后的文件输出到输入文件相同路径，以 `_mdnice.md` 为后缀；报错信息输出到 `_err.md` 后缀的文件内。

> 注意：只会替换本地图片文件的链接。如果没有需要上传至图床的本地图片文件，则不会输出新文件。

### 示例

使用 token 文件中的 JWT，将 `./test.md` 文件中的所有图片上传至图床，并获得替换图片链接后的新文件 `test.md_mdnice.md`：

```bash
# 原始 markdown 文档内容
$ cat test.md
![png](/contents/covers/backend-skill-tree.png)

[在线导图](https://www.processon.com/view/link/60f2d1b31efad41bbea9015e)
# 上传本地图片
# 可根据实际情况传入 img-path-prefix 参数，作为前缀加在 markdown 中图片 url 前面，用在无法直接根据 url 在本地文件系统找到对应图片文件的情况
# 如果图片 url 直接使用的图片文件的绝对或相对路径，此参数非必须
$ ./mdnice \
--token-file ./token \
--img-path-prefix /Users/alphahinex/github/origin/AlphaHinex.github.io/source \
-i test.md
2023/01/07 20:01:02 [DEBUG] Upload /Users/alphahinex/github/origin/AlphaHinex.github.io/source/contents/covers/backend-skill-tree.png to mdnice...
Write updated content to test.md_mdnice.md
# 查看替换图片链接后的 markdown 文档内容
$ cat test.md_mdnice.md
![png](https://files.mdnice.com/user/30377/d02b13c8-23a3-4df5-9b24-0ff9b2cac52f.png)

[在线导图](https://www.processon.com/view/link/60f2d1b31efad41bbea9015e)
```
