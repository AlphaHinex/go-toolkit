package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

var host string
var token string

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
			since := cCtx.String("since") + "T00:00:00"
			until := cCtx.String("until") + "T23:59:59"

			projectName, commitCount, err := getProjectInfo(projectId)
			if err != nil {
				return err
			}
			commits, err := getCommits(projectId, branch, since, until, commitCount)
			if err != nil {
				return err
			}
			//fmt.Printf("%s: %d", projectName, commitCount)
			fmt.Println("project,branch,sha,date,author,filename,filetype,operation,add,del,blankAdd,syntaxChange,spacingChange")
			for _, commit := range commits {
				diffs, err := getDiff(projectId, commit.ShortId)
				if err != nil {
					return err
				}
				for _, diff := range diffs {
					fmt.Printf("%s_%s,%s,%s,%s,%s,%s,%s,%s,%d,%d,%d,%d,%d\r\n",
						projectId, projectName, branch, commit.ShortId, commit.AuthoredDate, commit.AuthorEmail,
						diff.NewPath, diff.OldPath, "MODIFY", 1, 2, 3, 4, 5)
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
	Name       string
	Statistics projectStatistics `json:"statistics"`
}

type projectStatistics struct {
	CommitCount int `json:"commit_count"`
}

func getProjectInfo(projectId string) (string, int, error) {
	url := host + "/api/v4/projects/" + projectId + "?statistics=true"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Add("PRIVATE-TOKEN", token)

	res, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", 0, err
	}
	var response project
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", 0, err
	}
	return response.Name, response.Statistics.CommitCount, nil
}

type commit struct {
	ShortId      string `json:"short_id"`
	AuthorEmail  string `json:"author_email"`
	AuthoredDate string `json:"authored_date"`
}

type commits []commit

func getCommits(projectId, branch, since, until string, pageSize int) (commits, error) {
	url := host + "/api/v4/projects/" + projectId + "/repository/commits?ref_name=" + branch + "&since=" + since + "&until=" + until + "&per_page=" + strconv.Itoa(pageSize)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return nil, err
	}
	req.Header.Add("PRIVATE-TOKEN", token)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response commits
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
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
	url := host + "/api/v4/projects/" + projectId + "/repository/commits/" + commitShortId + "/diff"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return nil, err
	}
	req.Header.Add("PRIVATE-TOKEN", token)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response diffs
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
