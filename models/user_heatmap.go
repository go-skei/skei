// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"code.skei.dev/skei/modules/setting"
	"code.skei.dev/skei/modules/util"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     util.TimeStamp `json:"timestamp"`
	Contributions int64          `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData
func GetUserHeatmapDataByUser(user *User) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)
	var groupBy string
	var groupByName = "timestamp" // We need this extra case because mssql doesn't allow grouping by alias
	switch {
	case setting.UseSQLite3:
		groupBy = "strftime('%s', strftime('%Y-%m-%d', created_unix, 'unixepoch'))"
	case setting.UseMySQL:
		groupBy = "UNIX_TIMESTAMP(DATE(FROM_UNIXTIME(created_unix)))"
	case setting.UsePostgreSQL:
		groupBy = "extract(epoch from date_trunc('day', to_timestamp(created_unix)))"
	case setting.UseMSSQL:
		groupBy = "datediff(SECOND, '19700101', dateadd(DAY, 0, datediff(day, 0, dateadd(s, created_unix, '19700101'))))"
		groupByName = groupBy
	}

	sess := x.Select(groupBy+" AS timestamp, count(user_id) as contributions").
		Table("action").
		Where("user_id = ?", user.ID).
		And("created_unix > ?", (util.TimeStampNow() - 31536000))

	// * Heatmaps for individual users only include actions that the user themself
	//   did.
	// * For organizations actions by all users that were made in owned
	//   repositories are counted.
	if user.Type == UserTypeIndividual {
		sess = sess.And("act_user_id = ?", user.ID)
	}

	err := sess.GroupBy(groupByName).
		OrderBy("timestamp").
		Find(&hdata)

	return hdata, err
}
