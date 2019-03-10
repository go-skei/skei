// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"code.skei.dev/skei/models"
	"code.skei.dev/skei/models/migrations"
	"code.skei.dev/skei/modules/log"
	"code.skei.dev/skei/modules/setting"

	"github.com/urfave/cli"
)

// CmdMigrate represents the available migrate sub-command.
var CmdMigrate = cli.Command{
	Name:        "migrate",
	Usage:       "Migrate the database",
	Description: "This is a command for migrating the database, so that you can run gitea admin create-user before starting the server.",
	Action:      runMigrate,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration file path",
		},
	},
}

func runMigrate(ctx *cli.Context) error {
	if ctx.IsSet("config") {
		setting.CustomConf = ctx.String("config")
	}

	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	models.LoadConfigs()

	if err := models.NewEngine(migrations.Migrate); err != nil {
		log.Fatal(4, "Failed to initialize ORM engine: %v", err)
		return err
	}

	return nil
}
