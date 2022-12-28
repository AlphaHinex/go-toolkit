package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	app := &cli.App{
		Name:  "mdnice",
		Usage: "Upload pictures to mdnice",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "i",
				Value: ".",
				Usage: "Path to be uploaded",
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "Bearer token of mdnice",
			},
			&cli.StringFlag{
				Name:  "token-file",
				Usage: "Bearer token file of mdnice",
			},
		},
		Action: func(cCtx *cli.Context) error {
			src := cCtx.String("i")
			token := cCtx.String("token")
			if token == "" {
				tokenFilePath := cCtx.String("token-file")
				content, err := os.ReadFile(tokenFilePath)
				if err != nil {
					return err
				}
				token = string(content)
			}
			stat, _ := os.Stat(src)
			if stat.IsDir() {
				files, err := os.ReadDir(src)
				if err != nil {
					return err
				}

				md := ""
				errLog := ""
				for _, file := range files {
					if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
						continue
					}
					m, e := handleOneFile(src+string(filepath.Separator)+file.Name(), token)
					md += m
					errLog += e
				}
				if len(md) > 0 || len(errLog) > 0 {
					content := md + "\r\n---\r\n" + errLog
					err := os.WriteFile(src+string(filepath.Separator)+"README.md", []byte(content), 0666)
					if err != nil {
						return err
					}
				}
			} else {
				md, errLog := handleOneFile(src, token)
				if len(md) > 0 || len(errLog) > 0 {
					fmt.Println(md + "\r\n---\r\n" + errLog)
				}
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type mdniceRes struct {
	Code    int
	Message string
	Data    string
}

func upload(f string, token string) (string, error) {
	url := "https://api.mdnice.com/file/user/upload"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	file, _ := os.Open(f)
	defer file.Close()

	part1, err := writer.CreateFormFile("file", filepath.Base(f))
	_, err = io.Copy(part1, file)
	if err != nil {
		return "", err
	}
	err = writer.Close()
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var response mdniceRes
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}
	if response.Code == 0 {
		return response.Data, nil
	} else {
		return "", errors.New(strconv.Itoa(response.Code) + ":" + response.Message)
	}
}

func handleOneFile(filepath, token string) (string, string) {
	if strings.HasPrefix(filepath, ".") {
		return "", ""
	}
	if strings.HasSuffix(filepath, ".md") || strings.HasSuffix(filepath, ".markdown") {
		uploadImgInMarkdown(filepath, token)
		return "", ""
	} else {
		return uploadFile(filepath, token)
	}
}

func uploadFile(filepath, token string) (string, string) {
	md := ""
	errLog := ""
	link, err := upload(filepath, token)
	if err != nil {
		errLog = "Upload " + filepath + " failed: " + err.Error() + "\r\n"
		fmt.Printf("Failed to upload %s\r\n", filepath)
	} else {
		md = "![](" + link + ")\r\n"
		fmt.Printf("Upload %s done\r\n", filepath)
	}
	return md, errLog
}

func uploadImgInMarkdown(filepath, token string) {
	reader, _ := os.Open(filepath)
	buf := bufio.NewReader(reader)
	pattern := `!\[.*\]\((.*)\)`
	re := regexp.MustCompile(pattern)
	for {
		//遇到\n结束读取
		line, errR := buf.ReadString('\n')
		if errR == io.EOF {
			_ = reader.Close()
			break
		}
		for i, m := range re.FindStringSubmatch(line) {
			if i == 1 {
				fmt.Println(m)
			}
		}
	}
}
