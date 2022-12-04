package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "random-pick",
		Usage: "Random pick files in some path",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "n",
				Value: 10,
				Usage: "Pick n files",
			},
			&cli.StringFlag{
				Name:  "t",
				Value: "*",
				Usage: "File type to pick",
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
		},
		Action: func(cCtx *cli.Context) error {
			path := cCtx.String("i")
			files, err := os.ReadDir(path)
			if err != nil {
				log.Fatal(err)
			}

			output := cCtx.String("o")
			_, err = os.Stat(output)
			if os.IsNotExist(err) {
				err = os.MkdirAll(output, 0777)
				if err != nil {
					log.Fatal(err)
				}
			}

			n := cCtx.Int("n")
			t := cCtx.String("t")
			keep := cCtx.Bool("k")
			fileSet := make(map[string]struct{})
			rand.Seed(time.Now().UnixNano())
			for i := 0; i < n; {
				file := files[rand.Intn(len(files))]
				if !file.IsDir() && !strings.HasPrefix(file.Name(), ".") &&
					(t == "*" || strings.HasSuffix(strings.ToLower(file.Name()), t)) {
					if _, exist := fileSet[file.Name()]; exist {
						continue
					}
					fileSet[file.Name()] = struct{}{}
					from := path + "/" + file.Name()
					to := output + "/" + file.Name()
					op := "Copy"
					if !keep {
						err = os.Rename(from, to)
						if err != nil {
							log.Fatal(err)
						}
						op = "Move"
					} else {
						_, err = copyFile(from, to)
						if err != nil {
							log.Fatal(err)
						}
					}
					fmt.Printf("%s %s to %s\n", op, path+"/"+file.Name(), output+"/"+file.Name())
					i++
				}
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
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
