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
				Flags: backupOrRestoreFlags(),
				Action: func(c *cli.Context) error {
					cfg, err := loadValidatedConfig(c.String("config"))
					if err != nil {
						return err
					}

					return app.RunBackup(c.Context, cfg, c.Bool("verbose"))
				},
			},
			{
				Name:  "restore",
				Usage: "restore a backup into a configured database",
				Flags: append(
					backupOrRestoreFlags(),
					&cli.StringFlag{
						Name:  "db",
						Usage: "database name from config (optional; defaults to first database)",
					},
					&cli.StringFlag{
						Name:     "from",
						Required: true,
						Usage:    "path to backup file to restore",
					},
					&cli.BoolFlag{
						Name:  "clean",
						Usage: "drop database objects before recreating them (pg_restore --clean --if-exists)",
					},
					&cli.BoolFlag{
						Name:  "strict-sniff",
						Usage: "fail fast when backup header does not match restore config",
					},
					&cli.BoolFlag{
						Name:  "allow-sql-fallback",
						Usage: "if decoded stream is plain SQL, restore with psql instead of pg_restore",
					},
				),
				Action: func(c *cli.Context) error {
					cfg, err := loadValidatedConfig(c.String("config"))
					if err != nil {
						return err
					}

					return app.RunRestore(
						c.Context,
						cfg,
						c.String("db"),
						c.String("from"),
						c.Bool("verbose"),
						c.Bool("clean"),
						c.Bool("strict-sniff"),
						c.Bool("allow-sql-fallback"),
					)
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

func backupOrRestoreFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "config",
			Aliases:  []string{"c"},
			Required: true,
			Usage:    "path to config yaml",
		},
		&cli.BoolFlag{
			Name:  "verbose",
			Usage: "enable verbose logging",
		},
	}
}

func loadValidatedConfig(cfgPath string) (*config.Config, error) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
