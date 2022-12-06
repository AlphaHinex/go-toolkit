package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var host string
var token string

const pageSize = 99
const parallel = 10

func main() {
	app := &cli.App{
		Name:  "gitlab",
		Usage: "Use GitLab API to do some analysis works",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "url",
				Aliases:  []string{"u"},
				Usage:    "GitLab host url",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "access-token",
				Aliases:  []string{"t"},
				Usage:    "Access token to use GitLab API",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "project-id",
				Aliases: []string{"p"},
				Usage:   "Project ID in GitLab",
			},
			&cli.StringFlag{
				Name:     "branch",
				Aliases:  []string{"b"},
				Usage:    "Branch of project",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "since",
				Value: "2022-01-01",
				Usage: "Date of since, from 00:00:00",
			},
			&cli.StringFlag{
				Name:  "until",
				Value: "2022-12-31",
				Usage: "Date of until, to 23:59:59",
			},
		},
		Action: func(cCtx *cli.Context) error {
			host = cCtx.String("url")
			token = cCtx.String("access-token")
			projectId := cCtx.String("project-id")
			branch := cCtx.String("branch")
			since := cCtx.String("since")
			until := cCtx.String("until")

			projectName, err := getProjectInfo(projectId)
			if err != nil {
				return err
			}

			commitChannel := make(chan commit, 1000)
			go getCommits(projectId, branch, since+"T00:00:00", until+"T23:59:59", commitChannel)

			filename := fmt.Sprintf("%s_%s_%s_%s~%s.csv", projectId, projectName, branch, since, until)
			_ = os.Remove(filename)
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = file.WriteString("project,branch,sha,date,author,filename,filetype,operation,add,del,addIgnoreSpace,delIgnoreSpace\r\n")
			if err != nil {
				return err
			}

			rowChannel := make(chan string, 1000)
			go consumeCommit(projectId, projectName, branch, commitChannel, rowChannel)

			for row := range rowChannel {
				_, err = file.WriteString(row)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type project struct {
	Name string
}

func getProjectInfo(projectId string) (string, error) {
	url := host + "/api/v4/projects/" + projectId + "?statistics=true"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("PRIVATE-TOKEN", token)

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var response project
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}
	return response.Name, nil
}

type commits []commit

type commit struct {
	ShortId      string `json:"short_id"`
	AuthorEmail  string `json:"author_email"`
	AuthoredDate string `json:"authored_date"`
}

func getCommits(projectId, branch, since, until string, ch chan commit) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits?ref_name=%s&since=%s&until=%s&",
		host, projectId, branch, since, until)

	allData, err := getAllPageData(url)
	if err != nil {
		log.Fatal(err)
	}

	for _, data := range allData {
		var response commits
		err = json.Unmarshal(data, &response)
		if err != nil {
			log.Fatal(err)
		}
		for _, c := range response {
			ch <- c
		}
	}
	close(ch)
}

func consumeCommit(projectId, projectName, branch string, commitChannel chan commit, rowChannel chan string) {
	wg := sync.WaitGroup{}
	wg.Add(parallel)

	for i := 0; i < parallel; i++ {
		go func() {
			for c := range commitChannel {
				diffs, err := getDiff(projectId, c.ShortId)
				if err != nil {
					log.Fatal(err)
				}
				for _, diff := range diffs {
					op := "MODIFY"
					if diff.NewFile {
						op = "ADD"
					} else if diff.RenamedFile {
						op = "RENAME"
					} else if diff.DeletedFile {
						op = "DELETE"
					}
					add, del, actAdd, actDel := parseDiff(diff.Diff)
					rowChannel <- fmt.Sprintf("%s_%s,%s,%s,%s,%s,%s,%s,%s,%d,%d,%d,%d\r\n",
						projectId, projectName, branch, c.ShortId, c.AuthoredDate[0:10], c.AuthorEmail,
						diff.NewPath, filepath.Ext(diff.NewPath), op, add, del, actAdd, actDel)
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()
	close(rowChannel)
}

type diffs []diff

type diff struct {
	Diff        string `json:"diff"`
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

func getDiff(projectId, commitShortId string) (diffs, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits/%s/diff?", host, projectId, commitShortId)

	allData, err := getAllPageData(url)
	if err != nil {
		return nil, err
	}

	var result diffs
	for _, data := range allData {
		var response diffs
		err = json.Unmarshal(data, &response)
		if err != nil {
			return nil, err
		}
		result = append(result, response...)
	}
	return result, nil
}

func getAllPageData(url string) ([][]byte, error) {
	var allData [][]byte
	page := "1"
	for len(page) > 0 {
		data, p, err := getDataByPage(url, page)
		if err != nil {
			return nil, err
		}
		page = p
		allData = append(allData, data)
	}
	return allData, nil
}

func getDataByPage(url, page string) ([]byte, string, error) {
	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, fmt.Sprintf("%spage=%s&per_page=%d", url, page, pageSize), nil)

	if err != nil {
		return nil, "", err
	}
	req.Header.Add("PRIVATE-TOKEN", token)
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	return body, res.Header.Get("X-Next-Page"), nil
}

func parseDiff(d string) (int, int, int, int) {
	if len(d) == 0 {
		return 0, 0, 0, 0
	}

	addLines := 0
	delLines := 0
	addLinesIgnoreSpace := 0
	delLinesIgnoreSpace := 0

	rows := strings.Split(d, "\n")
	var add []string
	var del []string
	for idx, row := range rows {
		if idx == len(rows)-1 || (len(row) > 0 && row[0] == '@') {
			// compute first
			i0, i1, i2, i3 := computeLoC(add, del)
			addLines += i0
			delLines += i1
			addLinesIgnoreSpace += i2
			delLinesIgnoreSpace += i3
			// and then reset
			add = []string{}
			del = []string{}
			continue
		} else if len(row) == 0 {
			continue
		}

		c := ""
		if row[0] == '-' {
			c = strings.ReplaceAll(strings.ReplaceAll(strings.TrimLeft(row, "-"), " ", ""), "\r", "")
			if len(c) > 0 {
				del = append(del, c)
			} else {
				delLines++
			}
		} else if row[0] == '+' {
			c = strings.ReplaceAll(strings.ReplaceAll(strings.TrimLeft(row, "+"), " ", ""), "\r", "")
			if len(c) > 0 {
				add = append(add, c)
			} else {
				addLines++
			}
		}
	}

	return addLines, delLines, addLinesIgnoreSpace, delLinesIgnoreSpace
}

func computeLoC(add, del []string) (int, int, int, int) {
	addLinesIgnoreSpace := len(add)
	delLinesIgnoreSpace := len(del)

	for _, addContent := range add {
		for i, delContent := range del {
			if addContent == delContent {
				addLinesIgnoreSpace--
				delLinesIgnoreSpace--
				del[i] = "IGNORE_AT_" + strconv.FormatInt(time.Now().Unix(), 10) + del[i]
				break
			}
		}
	}
	//for _, row := range del {
	//	fmt.Println(row)
	//}

	return len(add), len(del), addLinesIgnoreSpace, delLinesIgnoreSpace
}
