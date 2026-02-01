package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:  "backupkit",
		Usage: "simple backups for local projects",
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "set up backup metadata for a project",
				Action: func(_ *cli.Context) error {
					fmt.Println("running init")
					return nil
				},
			},
			{
				Name:  "backup",
				Usage: "run a backup of the configured project",
				Action: func(_ *cli.Context) error {
					fmt.Println("running backup")
					return nil
				},
			},
			{
				Name:  "test",
				Usage: "verify backup configuration and targets",
				Action: func(_ *cli.Context) error {
					fmt.Println("running test")
					return nil
				},
			},
			{
				Name:  "daemon",
				Usage: "run backups on a schedule",
				Action: func(_ *cli.Context) error {
					fmt.Println("running daemon")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
