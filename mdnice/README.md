mdnice
======

将指定路径下的所有图片文件，上传至 [mdnice](https://editor.mdnice.com/) 的图床，需要 mdnice 的 JWT token。图片在图床的链接以 markdown 格式输出到图片来源路径的 README.md 文件中，上传失败的也会将失败原因记录至 md 文件。

```bash
$ ./mdnice -h
NAME:
   mdnice - Upload pictures to mdnice

USAGE:
   mdnice [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   -i value            Path to be uploaded (default: ".")
   --token value       Bearer token of mdnice
   --token-file value  Bearer token file of mdnice
   --help, -h          show help (default: false)
```

使用 token 文件中的 JWT Token，将 `./foo` 路径下的所有（图片）文件上传：

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
Upload ./foo/01670642460.GIF failed: 50005:文件过大
Upload ./foo/71670642460.PNG failed: 50005:文件过大
Upload ./foo/README.md failed: 50005:文件类型错误，仅支持jpg、jpeg、png、gif、svg类型
```
