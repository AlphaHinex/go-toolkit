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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var host string
var token string

const pageSize = 99

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
			&cli.IntFlag{
				Name:  "parallel",
				Value: 16,
				Usage: "Number of commit parsers",
			},
			&cli.StringFlag{
				Name:  "lark",
				Usage: "Lark webhook url",
			},
			&cli.IntFlag{
				Name:  "commit-parents",
				Value: -1,
				Usage: "Only count the commit has `commit-parents` number parent(s), \r\n" +
					"\t\t\t-1 means count all commits, \r\n" +
					"\t\t\t0 means only count the initial commit, \n" +
					"\t\t\t2 means only count merge request commit, \n" +
					"\t\t\t1 means exclude initial commit and merge request commit",
			},
		},
		Action: func(cCtx *cli.Context) error {
			from := time.Now()
			host = cCtx.String("url")
			token = cCtx.String("access-token")
			projectId := cCtx.String("project-id")
			branch := cCtx.String("branch")
			since := cCtx.String("since")
			until := cCtx.String("until")
			parents := cCtx.Int("commit-parents")

			proj, err := getProjectInfo(projectId)
			if err != nil {
				return err
			}
			log.Printf("Start to analyse %s ...\r\n", proj.Name)

			commitChannel := make(chan commit, 1000)
			go getCommits(projectId, branch, since+"T00:00:00", until+"T23:59:59", commitChannel, parents)

			filename := fmt.Sprintf("%s_%s_%s_%s~%s.csv", projectId, proj.Name, branch, since, until)
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

			parallel := cCtx.Int("parallel")
			rowChannel := make(chan string, 1000)
			statChannel := make(chan map[string]*stat, parallel)
			go consumeCommit(projectId, proj.Name, branch, parallel, commitChannel, rowChannel, statChannel)

			for row := range rowChannel {
				_, err = file.WriteString(row)
				if err != nil {
					return err
				}
			}
			log.Printf("Generate %s use %s.\r\n", filename, time.Since(from))

			userStat := make(map[string]*stat)
			for us := range statChannel {
				for user, s := range us {
					if _, exist := userStat[user]; exist {
						userStat[user].add += s.add
						userStat[user].del += s.del
						userStat[user].addIgnoreSpace += s.addIgnoreSpace
						userStat[user].delIgnoreSpace += s.delIgnoreSpace
						userStat[user].fileCount += s.fileCount
						userStat[user].commitCount += s.commitCount
					} else {
						userStat[user] = s
					}
				}
			}

			results := getResults(userStat)
			sort.Sort(results)

			title := fmt.Sprintf("%s 项目 %s  分支代码分析结果（%s~%s)", proj.Name, branch, since, until)
			content := fmt.Sprintf("No. %-25s effLines(ratio)\teffAdds(ratio)\tcommits\tfiles\r\n", "author")
			for i, r := range results {
				content += fmt.Sprintf("%2d. %-25s %d(%.2f%%)\t%d(%.2f%%)\t%d\t%d\r\n", i+1, r.email,
					r.addIgnoreSpace+r.delIgnoreSpace, float32(r.addIgnoreSpace+r.delIgnoreSpace)/float32(r.add+r.del)*100,
					r.addIgnoreSpace, float32(r.addIgnoreSpace)/float32(r.add)*100,
					r.commitCount, r.fileCount)
			}
			cp := "统计了所有 Commit"
			switch parents {
			case 2:
				cp = "仅统计 Merge Request Commit"
				break
			case 1:
				cp = "统计了除初始 Commit 和 Merge Request 外的所有 Commit"
				break
			case 0:
				cp = "仅统计了初始 Commit"
				break
			default:
				cp = "统计了 Parent 数量为 " + strconv.Itoa(parents) + " 的 Commit"
			}
			desc := `以上结果` + cp + `（时间范围内）
* effLines（有效代码行数）= 有效增加代码行数 + 有效减少代码行数
* effLines ratio（有效代码率）= 有效代码行数 / 总代码行数 * 100%
* effAdds（有效增加行数）= 有效增加代码行数
* effAdds ratio（有效增加率）= 有效增加代码行数 / 总增加代码行数 * 100%
* commits：Commit 总数
* files：文件总数（不去重）
* 有效代码：忽略仅有空格或换行的代码改动，diff -w`

			lark := cCtx.String("lark")
			if len(lark) > 0 {
				sendLarkMsg(lark, proj.WebUrl, title, content, desc)
			}

			fmt.Printf("\r\n%s\r\n\r\n%s\r\n%s\r\n", title, content, desc)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func sendLarkMsg(url, projectUrl, title, content, desc string) {
	text := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(content+desc, "\t", "\\t"), "\r", "\\r"), "\n", "\\n")
	payload := strings.NewReader(`{
    "msg_type": "post",
    "content": {
        "post": {
            "zh_cn": {
                "title": "` + title + `",
                "content": [
                    [
                        {
                            "tag": "text",
                            "text": "` + text + `"
                        }
                    ],
                    [
                        {
                            "tag": "a",
                            "text": "GitLab 仓库地址",
                            "href": "` + projectUrl + `"
                        }
                    ]
                ]
            }
        }
    } 
}
`)

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, payload)

	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	log.Printf("Response from lark: %s\r\n", string(body))
}

type project struct {
	Name   string
	WebUrl string `json:"web_url"`
}

func getProjectInfo(projectId string) (project, error) {
	url := host + "/api/v4/projects/" + projectId + "?statistics=true"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return project{}, err
	}
	req.Header.Add("PRIVATE-TOKEN", token)

	res, err := client.Do(req)
	if err != nil {
		return project{}, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return project{}, err
	}
	var response project
	err = json.Unmarshal(body, &response)
	if err != nil {
		return project{}, err
	}
	return response, nil
}

type commits []commit

type commit struct {
	ShortId      string   `json:"short_id"`
	AuthorEmail  string   `json:"author_email"`
	AuthoredDate string   `json:"authored_date"`
	ParentIds    []string `json:"parent_ids"`
}

func getCommits(projectId, branch, since, until string, ch chan commit, parents int) {
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
			if parents > -1 {
				if len(c.ParentIds) != parents {
					continue
				}
			}
			ch <- c
		}
	}
	close(ch)
	log.Println("Load all commits")
}

func consumeCommit(projectId, projectName, branch string, parallel int,
	commitChannel chan commit, rowChannel chan string, statChannel chan map[string]*stat) {
	wg := sync.WaitGroup{}
	wg.Add(parallel)

	for i := 0; i < parallel; i++ {
		go func() {
			userMap := make(map[string]*stat)
			for c := range commitChannel {
				if _, exist := userMap[c.AuthorEmail]; !exist {
					userMap[c.AuthorEmail] = &stat{
						email: c.AuthorEmail,
					}
				}
				user := userMap[c.AuthorEmail]
				user.commitCount++
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
					user.fileCount++
					user.add += add
					user.del += del
					user.addIgnoreSpace += actAdd
					user.delIgnoreSpace += actDel
					rowChannel <- fmt.Sprintf("%s_%s,%s,%s,%s,%s,%s,%s,%s,%d,%d,%d,%d\r\n",
						projectId, projectName, branch, c.ShortId, toCSTStr(c.AuthoredDate), c.AuthorEmail,
						diff.NewPath, filepath.Ext(diff.NewPath), op, add, del, actAdd, actDel)
				}
			}
			statChannel <- userMap
			wg.Done()
		}()
	}

	wg.Wait()
	close(rowChannel)
	close(statChannel)
}

func toCSTStr(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		log.Fatal(err)
	}

	// 将解析出来的时间设置为东八区
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t = t.In(loc)
	return t.Format("2006-01-02 15:04:05")
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

type stat struct {
	email          string
	add            int
	del            int
	addIgnoreSpace int
	delIgnoreSpace int
	commitCount    int
	fileCount      int
}

type Results []stat

func getResults(userStat map[string]*stat) Results {
	var results Results
	for _, v := range userStat {
		results = append(results, *v)
	}
	return results
}

func (re Results) Len() int { return len(re) }

func (re Results) Swap(i, j int) { re[i], re[j] = re[j], re[i] }

func (re Results) Less(i, j int) bool {
	if re[i].addIgnoreSpace+re[i].delIgnoreSpace < re[j].addIgnoreSpace+re[j].delIgnoreSpace {
		return false
	} else if re[i].addIgnoreSpace+re[i].delIgnoreSpace == re[j].addIgnoreSpace+re[j].delIgnoreSpace {
		if re[i].add+re[i].del > re[j].add+re[j].del {
			return false
		} else if re[i].add+re[i].del == re[j].add+re[j].del {
			if re[i].fileCount < re[j].fileCount {
				return false
			}
		}
	}
	return true
}
