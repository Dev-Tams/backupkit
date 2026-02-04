package main

import (
	"fmt"
	"os"

	"github.com/dev-tams/backupkit/internal/app"
	"github.com/dev-tams/backupkit/internal/config"
	"github.com/urfave/cli/v2"
)

func main() {
	// CLI entrypoint wiring; commands are stubbed for now.
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
				Action: func(c *cli.Context) error {
					cfgPath := c.String("config")
					cfg, err := config.LoadConfig(cfgPath)
					if err != nil {
						return err
					}
					app.RunBackup(c.Context, cfg)
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
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "path to config yaml",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
