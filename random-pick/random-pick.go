package main

import (
	"fmt"
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
				Name:  "i",
				Value: ".",
				Usage: "Path to pick files",
			},
			&cli.StringFlag{
				Name:  "t",
				Value: "*",
				Usage: "File type to pick",
			},
		},
		Action: func(cCtx *cli.Context) error {
			path := cCtx.String("i")
			files, err := os.ReadDir(path)
			if err != nil {
				log.Fatal(err)
			}

			n := cCtx.Int("n")
			t := cCtx.String("t")
			rand.Seed(time.Now().UnixNano())
			for i := 0; i < n; {
				file := files[rand.Intn(len(files))]
				if !file.IsDir() && (t == "*" || strings.HasSuffix(strings.ToLower(file.Name()), t)) {
					fmt.Println(file.Name())
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
