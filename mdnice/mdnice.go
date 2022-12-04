package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
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
			files, err := os.ReadDir(src)
			if err != nil {
				return err
			}

			md := ""
			errlog := ""
			for _, file := range files {
				if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
					continue
				}
				link, err := upload(src+string(filepath.Separator)+file.Name(), token)
				if err != nil {
					errlog += "Upload " + src + string(filepath.Separator) + file.Name() + " failed: " + err.Error() + "\r\n"
					fmt.Printf("Failed to upload %s\r\n", file.Name())
				} else {
					md += "![](" + link + ")\r\n"
					fmt.Printf("Upload %s done\r\n", file.Name())
				}
			}
			content := md + "\r\n---\r\n" + errlog
			err = os.WriteFile(src+string(filepath.Separator)+"README.md", []byte(content), 0666)
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
