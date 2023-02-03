package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/antchfx/xmlquery"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

var snapshot, release string

func main() {
	app := &cli.App{
		Name: "批量上传 Jar 包及同名 pom 文件（如果存在）至 Maven 仓库工具。",
		UsageText: `upload-jars [-i 查找 Jar 包的根路径] [-c 配置文件] [-s snapshot 仓库 url] [-r release 仓库 url]

默认从命令执行路径查找要上传的 Jar 包。
查找 Jar 包的根路径中需包含按 GAV（group、artifact、version）创建的三级路径，
Jar 包及 pom 文件（如果存在）放在 version 路径内，如：

├── com.alibaba
│   └── druid
│       └── 2.5.8
│           └── test.jar
├── org.codehaus.groovy
│   ├── groovy-console
│   │   ├── 2.5.8
│   │   │   ├── test-snapshot.jar
│   │   │   ├── test-snapshot.pom
│   │   │   ├── test.jar
│   │   │   └── test.pom
│   │   └── 2.5.9
│   │       └── test.jar
│   └── groovy-shell
│       ├── 2.5.8
│       │   ├── test-snapshot.jar
│       │   ├── test-snapshot.pom
│       │   ├── test.jar
│       │   └── test.pom
│       └── 2.5.9
│           └── test.jar

Maven 仓库地址需通过命令行参数或配置文件指定，命令行参数会覆盖配置文件中对应地址。
仓库地址需指定两个，一个 snapshot 仓库，一个 release 仓库。
工具根据 Jar 包文件名中是否包含 snapshot（不区分大小写）关键字进行区分，
包含上传至 snapshot 仓库，不包含上传至 release 仓库。
Maven 仓库上传需要身份认证时，需将认证信息包含进仓库地址中，格式如下：
http://username:pwd@host:port/path/to/repository
其中用户名、密码如包含特殊字符，需进行 URL 转义。

源码可见：https://github.com/AlphaHinex/go-toolkit/tree/main/upload-jars`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "i",
				Value: ".",
				Usage: "查找 Jar 包的根路径，默认为当前路径",
			},
			&cli.StringFlag{
				Name:  "s",
				Usage: "snapshot 仓库 url",
			},
			&cli.StringFlag{
				Name:  "r",
				Usage: "release 仓库 url",
			},
			&cli.StringFlag{
				Name: "c",
				Usage: `Properties 配置文件，格式如下：
snapshot=http://username:pwd@host:port/path/to/snapshot-repository
release=http://username:pwd@host:port/path/to/release-repository
`,
			},
		},
		Action: func(cCtx *cli.Context) error {
			inputPath := cCtx.String("i")

			snapshot = cCtx.String("s")
			release = cCtx.String("r")
			configPath := cCtx.String("c")
			if (len(snapshot) == 0 || len(release) == 0) && len(configPath) > 0 {
				content, err := os.ReadFile(configPath)
				if err != nil {
					return err
				}
				for _, property := range strings.Split(string(content), "\n") {
					if strings.HasPrefix(property, "snapshot=") {
						snapshot = strings.TrimPrefix(property, "snapshot=")
					} else if strings.HasPrefix(property, "release=") {
						release = strings.TrimPrefix(property, "release=")
					}
				}
				if len(snapshot) == 0 || len(release) == 0 {
					return errors.New("必须指定上传仓库地址")
				}
			} else {
				return errors.New("必须指定上传仓库地址")
			}

			err := uploadJarsInGavFolders(inputPath)
			if err != nil {
				return err
			}
			err = uploadJarsInInputPath(inputPath)
			if err != nil {
				return err
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func uploadJarsInGavFolders(inputPath string) error {
	groupDirs, err := os.ReadDir(inputPath)
	if err != nil {
		return err
	}
	for _, groupDir := range groupDirs {
		if !groupDir.IsDir() {
			continue
		}
		artifactDirs, err := os.ReadDir(filepath.Join(inputPath, groupDir.Name()))
		if err != nil {
			return err
		}
		for _, artifactDir := range artifactDirs {
			if !artifactDir.IsDir() {
				continue
			}
			versionDirs, err := os.ReadDir(filepath.Join(inputPath, groupDir.Name(), artifactDir.Name()))
			if err != nil {
				return err
			}
			for _, versionDir := range versionDirs {
				if !versionDir.IsDir() {
					continue
				}
				entries, err := os.ReadDir(filepath.Join(inputPath, groupDir.Name(), artifactDir.Name(), versionDir.Name()))
				if err != nil {
					return err
				}
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jar") {
						jarFilePath, _ := filepath.Abs(
							filepath.Join(inputPath, groupDir.Name(), artifactDir.Name(), versionDir.Name(), entry.Name()))
						pomFilePart, _ := filepath.Abs(
							filepath.Join(inputPath, groupDir.Name(), artifactDir.Name(), versionDir.Name(),
								strings.ReplaceAll(entry.Name(), ".jar", ".pom")))
						_, err = os.Stat(pomFilePart)
						if os.IsNotExist(err) {
							pomFilePart = ""
						} else {
							pomFilePart = fmt.Sprintf("-DpomFile=%s ", pomFilePart)
						}
						url := release
						if strings.Contains(strings.ToLower(entry.Name()), "snapshot") {
							url = snapshot
						}
						//mvn deploy:deploy-file \
						//-DgroupId=org.codehaus.groovy \
						//-DartifactId=groovy-console \
						//-Dversion=2.5.8 \
						//-Dpackaging=jar \
						//-DpomFile=/path/to/groovy-console-2.5.8.pom \
						//-Dfile=/path/to/groovy-console-2.5.8.jar \
						//-Durl=http://username:password@ip:host/repository/repo-releases/
						execute(fmt.Sprintf("mvn deploy:deploy-file "+
							"-DgroupId=%s -DartifactId=%s -Dversion=%s "+
							"-Dpackaging=jar %s-Dfile=%s -Durl=%s\r\n",
							groupDir.Name(), artifactDir.Name(), versionDir.Name(), pomFilePart, jarFilePath, url))
					}
				}
			}
		}
	}
	return nil
}

func execute(command string) {
	strs := strings.Split(command, " ")
	cmd := exec.Command(strs[0], strs[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout // 标准输出
	cmd.Stderr = &stderr // 标准错误
	err := cmd.Run()
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	fmt.Printf("%s%s", outStr, errStr)
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\ncomomand: %s\n", err, command)
	}
}

func uploadJarsInInputPath(inputPath string) error {
	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pom") {
			continue
		}
		javaFilePath, _ := filepath.Abs(filepath.Join(inputPath, strings.ReplaceAll(entry.Name(), ".pom", ".jar")))
		_, err = os.Stat(javaFilePath)
		if os.IsNotExist(err) {
			continue
		}

		pomFilePath, err := filepath.Abs(filepath.Join(inputPath, entry.Name()))
		if err != nil {
			return err
		}
		f, err := os.Open(pomFilePath)
		if err != nil {
			return err
		}
		doc, err := xmlquery.Parse(f)
		if err != nil {
			return err
		}
		groupId := getGav(doc, "groupId")
		artifactId := getGav(doc, "artifactId")
		version := getGav(doc, "version")

		url := release
		if strings.Contains(strings.ToLower(entry.Name()), "snapshot") {
			url = snapshot
		}

		execute(fmt.Sprintf("mvn deploy:deploy-file "+
			"-DgroupId=%s -DartifactId=%s -Dversion=%s "+
			"-Dpackaging=jar -DpomFile=%s -Dfile=%s -Durl=%s\r\n",
			groupId, artifactId, version, pomFilePath, javaFilePath, url))
	}
	return nil
}

func getGav(doc *xmlquery.Node, tag string) string {
	gav, err := xmlquery.Query(doc, fmt.Sprintf("//project/%s", tag))
	if err != nil || gav == nil {
		gav = xmlquery.FindOne(doc, fmt.Sprintf("//project/parent/%s", tag))
	}
	return gav.InnerText()
}
