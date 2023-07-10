package main

import (
	"encoding/json"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var channelBuffer = 100
var includedFiletypes = ""
var filesChannel = make(chan string, channelBuffer)
var converterParallel = 8

func main() {
	app := &cli.App{
		Name:    "files2jsonl",
		Usage:   "Convert selected files into one JSON lines file",
		Version: "v2.2.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "dir",
				Aliases:  []string{"d"},
				Usage:    "Path to pick files",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "include",
				Aliases: []string{"i"},
				Usage: "Only convert included file types, " +
					"comma separated for multi values: 'jpg,png', case insensitive, " +
					"not set means include all types",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   ".",
				Usage:   "Output JSON lines file's directory",
			},
		},
		Action: func(cCtx *cli.Context) error {
			inputDir := cCtx.String("dir")
			includedFiletypes = cCtx.String("include")
			outputDir := cCtx.String("output")

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := filepath.WalkDir(inputDir, loadFilteredFiles)
				if err != nil {
					log.Fatal(err)
				}
				close(filesChannel)
			}()

			rowChannel := make(chan string, channelBuffer)
			go func() {
				for i := 0; i < converterParallel; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						convertFile2Json(rowChannel)
					}()
				}
				wg.Wait()
				close(rowChannel)
			}()

			f, err := os.OpenFile(filepath.Join(outputDir, "data.jsonl"), os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			for s := range rowChannel {
				_, err = f.WriteString(s)
				if err != nil {
					log.Fatal(err)
				}
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Run app failed: %s", err)
	}
}

func loadFilteredFiles(path string, entry os.DirEntry, _ error) error {
	if entry.IsDir() {
		return nil
	}

	slice := strings.Split(strings.ToLower(entry.Name()), ".")
	fileType := slice[len(slice)-1]
	if includedFiletypes == "" || strings.Contains(includedFiletypes, fileType) {
		filesChannel <- path
	}
	return nil
}

type jsonRow struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

func convertFile2Json(rowChannel chan string) {
	for filePath := range filesChannel {
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Read %s error: %s", filePath, err)
		}

		row := jsonRow{Text: string(content), URL: filePath}
		rowByte, err := json.Marshal(row)
		if err != nil {
			log.Fatalf("Marshal %s error: %s", row, err)
		}
		rowChannel <- string(rowByte) + "\r\n"
	}
}
