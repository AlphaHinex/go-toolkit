package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

var host string
var token string

func main() {
	app := &cli.App{
		Name:    "sonar-exp",
		Usage:   "Export sonar projects info into csv",
		Version: "v2.2.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "host",
				Usage:    "Sonar host, http://localhost:9000 for example",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "token",
				Aliases: []string{"t"},
				Usage: "User token, could get follow " +
					"https://docs.sonarsource.com/sonarqube/latest/user-guide/user-account/generating-and-using-tokens/",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Filter projects by query string",
			},
		},
		Action: func(cCtx *cli.Context) error {
			host = cCtx.String("host")
			token = cCtx.String("token")
			query := cCtx.String("query")

			projects, err := getAllProjects(query)
			if err != nil {
				return err
			}
			err = printCsv(projects)
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

func getAllProjects(query string) ([]string, error) {
	var allProjectKeys []string
	page := 1
	for page > 0 {
		keys, hasNext, err := getProjectsByPage(page, query)
		if err != nil {
			return nil, err
		}
		if hasNext {
			page++
		} else {
			page = 0
		}
		allProjectKeys = append(allProjectKeys, keys...)
	}
	return allProjectKeys, nil
}

func getProjectsByPage(page int, query string) ([]string, bool, error) {
	filter := ""
	if len(query) > 0 {
		filter = "&filter=query%20%3D%20%22" + query + "%22"
	}

	url := fmt.Sprintf("%s/api/components/search_projects?p=%d%s", host, page, filter)
	body, err := get(url)
	var response project
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Printf("Parse %s error: %s", string(body), err)
		return nil, false, err
	}

	total := response.Paging.Total
	countCurrent := len(response.Components)
	countBefore := (response.Paging.PageIndex - 1) * response.Paging.PageSize

	var keys []string
	for _, component := range response.Components {
		keys = append(keys, component.Key)
	}

	return keys, total > countBefore+countCurrent, nil
}

type project struct {
	Paging struct {
		PageIndex int `json:"pageIndex"`
		PageSize  int `json:"pageSize"`
		Total     int `json:"total"`
	} `json:"paging"`
	Components []struct {
		Key string `json:"key"`
	} `json:"components"`
}

func get(url string) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(token+":")))
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

func getProjectMeasures(key string) (measures, error) {
	url := fmt.Sprintf("%s/api/measures/search?projectKeys=%s", host, key)
	url += "&metricKeys=bugs%2Cvulnerabilities%2Csecurity_hotspots_reviewed%2Ccode_smells%2Cduplicated_lines_density%2Ccoverage%2Cncloc%2Cncloc_language_distribution"
	body, err := get(url)
	var response measures
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Printf("Parse %s error: %s", string(body), err)
		return measures{}, err
	}
	return response, nil
}

type measures struct {
	Measures []struct {
		Metric    string `json:"metric"`
		Value     string `json:"value"`
		Component string `json:"component"`
	} `json:"measures"`
}

func printCsv(projects []string) error {
	fmt.Printf("Project,Bugs,Vulnerabilities,Hotspots Reviewed,Code Smells,Coverage,Duplications,Lines,NCLOC Language Distribution\n")

	dict := make(map[string]int, 8)
	dict["bugs"] = 1
	dict["vulnerabilities"] = 2
	dict["security_hotspots_reviewed"] = 3
	dict["code_smells"] = 4
	dict["coverage"] = 5
	dict["duplicated_lines_density"] = 6
	dict["ncloc"] = 7
	dict["ncloc_language_distribution"] = 8

	for _, key := range projects {
		m, err := getProjectMeasures(key)
		if err != nil {
			return err
		}
		line := []string{"-", "-", "-", "-", "-", "-", "-", "-", "-"}
		for j, measure := range m.Measures {
			if j == 0 {
				line[0] = measure.Component
			}
			line[dict[measure.Metric]] = measure.Value
		}
		fmt.Printf("%s\n", strings.Join(line, ","))
	}
	return nil
}