package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

var quiet = false

func main() {
	app := &cli.App{
		Name:    "random-pick",
		Usage:   "Random pick files in some path",
		Version: "v2.4.0",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "n",
				Value: 10,
				Usage: "Pick n files",
			},
			&cli.StringFlag{
				Name:  "t",
				Value: "*",
				Usage: "File type(s) to pick, * means all types, " +
					"comma separated for multi values: 'jpg,png', case insensitive",
			},
			&cli.StringFlag{
				Name:  "i",
				Value: ".",
				Usage: "Path to pick files",
			},
			&cli.StringFlag{
				Name:  "o",
				Value: ".",
				Usage: "Output picked files",
			},
			&cli.BoolFlag{
				Name:  "k",
				Value: false,
				Usage: "Keep picked files in path",
			},
			&cli.BoolFlag{
				Name:  "q",
				Value: false,
				Usage: "Be quiet, not print anything",
			},
		},
		Action: func(cCtx *cli.Context) error {
			input := cCtx.String("i")
			filter := cCtx.String("t")
			quiet = cCtx.Bool("q")
			files, err := loadFiles(input, filter)
			if err != nil && !quiet {
				log.Fatal(err)
			}

			output := cCtx.String("o")
			_, err = os.Stat(output)
			if os.IsNotExist(err) {
				err = os.MkdirAll(output, 0777)
				if err != nil && !quiet {
					log.Fatal(err)
				}
			}

			n := cCtx.Int("n")
			keep := cCtx.Bool("k")
			fileSet := make(map[string]struct{})
			rand.Seed(time.Now().UnixNano())
			if n > len(files) {
				n = len(files)
			}
			for i := 0; i < n; {
				file := files[rand.Intn(len(files))]
				if _, exist := fileSet[file.Name()]; exist {
					continue
				}
				fileSet[file.Name()] = struct{}{}
				from := input + string(filepath.Separator) + file.Name()
				to := output + string(filepath.Separator) + strconv.Itoa(i) + strconv.FormatInt(time.Now().Unix(), 10) + filepath.Ext(file.Name())
				op := "Pick"
				if input == output {
					// Do nothing
				} else if !keep {
					err = os.Rename(from, to)
					if err != nil && !quiet {
						log.Fatal(err)
					}
					op = "Move"
				} else {
					_, err = copyFile(from, to)
					if err != nil && !quiet {
						log.Fatal(err)
					}
					op = "Copy"
				}
				if !quiet {
					fmt.Printf("%s %s to %s\n", op, from, to)
				}
				i++
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil && !quiet {
		log.Fatal(err)
	}
}

func loadFiles(src, filter string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(src)
	var result []fs.DirEntry
	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		include := false
		types := strings.Split(filter, ",")
		for _, t := range types {
			if t == "*" {
				include = true
			} else if strings.HasSuffix(strings.ToLower(file.Name()), strings.TrimSpace(t)) {
				include = true
			}
		}
		if include {
			result = append(result, file)
		}
	}
	return result, err
}

func copyFile(srcFile, destFile string) (int64, error) {
	file1, err := os.Open(srcFile)
	if err != nil {
		return 0, err
	}
	file2, err := os.OpenFile(destFile, os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		return 0, err
	}
	defer file1.Close()
	defer file2.Close()

	return io.Copy(file2, file1)
}
