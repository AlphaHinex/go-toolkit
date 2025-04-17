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
	"time"
)

var quiet = false

func main() {
	app := &cli.App{
		Name:    "mdnice",
		Usage:   "Upload pictures to mdnice",
		Version: "v2.4.1",
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
			&cli.StringFlag{
				Name:  "img-path-prefix",
				Usage: "Path to add before image link (local file path) in markdown file",
			},
			&cli.BoolFlag{
				Name:  "q",
				Value: false,
				Usage: "Be quiet, not print anything",
			},
		},
		Action: func(cCtx *cli.Context) error {
			src := cCtx.String("i")
			token := cCtx.String("token")
			quiet = cCtx.Bool("q")
			imgPathPrefix := cCtx.String("img-path-prefix")
			if token == "" {
				tokenFilePath := cCtx.String("token-file")
				content, err := os.ReadFile(tokenFilePath)
				if err != nil {
					return err
				}
				token = strings.Split(string(content), "\n")[0]
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
					m, e := handleOneFile(src+string(filepath.Separator)+file.Name(), token, imgPathPrefix)
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
				md, errLog := handleOneFile(src, token, imgPathPrefix)
				if (len(md) > 0 || len(errLog) > 0) && !quiet {
					fmt.Println(md + "\r\n---\r\n" + errLog)
				}
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil && !quiet {
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

func handleOneFile(filepath, token, imgPathPrefix string) (string, string) {
	if strings.HasPrefix(filepath, ".") {
		return "", ""
	}
	if strings.HasSuffix(filepath, ".md") || strings.HasSuffix(filepath, ".markdown") {
		uploadImgInMarkdown(filepath, token, imgPathPrefix)
		return "", ""
	} else {
		retry := 2
		link, err := upload(filepath, token)
		if err != nil {
			for retry > 0 {
				retry--
				time.Sleep(1 * time.Second)
				link, err = upload(filepath, token)
				if err == nil {
					break
				} else {
					continue
				}
			}

		}
		if retry == 0 && err != nil {
			if !quiet {
				fmt.Printf("Failed to upload %s\r\n", filepath)
			}
			return "", "1. Upload " + filepath + " failed: " + err.Error() + "\r\n"
		} else {
			if !quiet {
				fmt.Printf("Upload %s done\r\n", filepath)
			}
			return "![](" + link + ")\r\n", ""
		}
	}
}

func uploadImgInMarkdown(mdFilePath, token, imgPathPrefix string) {
	newMarkdown, errLogs, updated := "", "", false

	reader, _ := os.Open(mdFilePath)
	buf := bufio.NewReader(reader)
	pattern := `!\[.*\]\((.*)\)`
	re := regexp.MustCompile(pattern)
	for {
		//遇到\n结束读取
		line, errR := buf.ReadString('\n')
		if re.MatchString(line) {
			for i, m := range re.FindStringSubmatch(line) {
				if i == 1 {
					imgPath := m
					if strings.HasPrefix(imgPath, "http") {
						newMarkdown += line
					} else {
						absPath, err := filepath.Abs(imgPath)
						if err != nil {
							if !quiet {
								log.Printf("[WARN] Could not get abs path of %s.\r\n", imgPath)
							}
							absPath = imgPath
						}
						if !quiet {
							log.Printf("[DEBUG] Upload %s to mdnice...", imgPathPrefix+absPath)
						}
						link, err := upload(imgPathPrefix+absPath, token)
						if err != nil {
							newMarkdown += line
							errLogs += fmt.Sprintf("1. Upload %s failed with error: %s\r\n", imgPath, err)
						} else {
							newMarkdown += strings.ReplaceAll(line, imgPath, link)
							updated = true
						}
					}
				}
			}
		} else {
			newMarkdown += line
		}
		if errR == io.EOF {
			_ = reader.Close()
			break
		}
	}

	if updated {
		_ = os.WriteFile(mdFilePath+"_mdnice.md", []byte(newMarkdown), 0666)
		if !quiet {
			fmt.Printf("Write updated content to %s", mdFilePath+"_mdnice.md")
		}
	} else if !quiet {
		fmt.Printf("Nothing changed in %s", mdFilePath)
	}
	if len(errLogs) > 0 {
		_ = os.WriteFile(mdFilePath+"_err.md", []byte(errLogs), 0666)
		if !quiet {
			fmt.Printf(" with errors in %s", mdFilePath+"_err.md")
		}
	}
	if !quiet {
		fmt.Println()
	}
}
