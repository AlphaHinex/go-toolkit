package main

import (
	"bytes"
	"fmt"
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
		Name:  "upload-jars",
		Usage: "Upload jars to maven repository",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "i",
				Value: ".",
				Usage: "Input path of jars",
			},
			&cli.StringFlag{
				Name:     "s",
				Required: true,
				Usage:    "Maven snapshot repository URL to upload",
			},
			&cli.StringFlag{
				Name:     "r",
				Required: true,
				Usage:    "Maven release repository URL to upload",
			},
		},
		Action: func(cCtx *cli.Context) error {
			inputPath := cCtx.String("i")
			snapshot = cCtx.String("s")
			release = cCtx.String("r")
			err := uploadJarsInGavFolders(inputPath)
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
