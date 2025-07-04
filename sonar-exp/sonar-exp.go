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
	"strconv"
	"strings"
)

var host string
var token string

func main() {
	app := &cli.App{
		Name:    "sonar-exp",
		Usage:   "Export sonar projects info into csv",
		Version: "v2.5.1",
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
	fmt.Println("Project," +
		"Bugs,Vulnerabilities,Hotspots Reviewed,Code Smells,Coverage,Duplications,Lines,NCLOC Language Distribution," +
		"Size,Duplications*Lines,Bug/Lines*1k%,Code Smells/Lines*1k%")

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
		computed, err := getComputedValues(line, dict)
		if err != nil {
			return err
		}
		fmt.Printf("%s,%s\n", strings.Join(line, ","), strings.Join(computed, ","))
	}
	return nil
}

func getComputedValues(line []string, dict map[string]int) ([]string, error) {
	computed := []string{"-", "-", "-", "-"}

	lines, err := strconv.Atoi(line[dict["ncloc"]])
	if err != nil || lines == 0 {
		return computed, nil
	}

	if lines > 500_000 {
		computed[0] = "XL"
	} else if lines > 100_000 {
		computed[0] = "L"
	} else if lines > 10_000 {
		computed[0] = "M"
	} else if lines > 1_000 {
		computed[0] = "S"
	} else {
		computed[0] = "XS"
	}

	duplications, err := strconv.ParseFloat(line[dict["duplicated_lines_density"]], 32)
	if err == nil {
		computed[1] = fmt.Sprintf("%f", float32(duplications)*float32(lines)/100)
	}

	bug, err := strconv.Atoi(line[dict["bugs"]])
	if err == nil {
		computed[2] = fmt.Sprintf("%f", float32(bug)/float32(lines)*1000)
	}

	codeSmells, err := strconv.Atoi(line[dict["code_smells"]])
	if err == nil {
		computed[3] = fmt.Sprintf("%f", float32(codeSmells)/float32(lines)*1000)
	}

	return computed, nil
}
