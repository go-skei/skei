// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/context"
)

// IssueStopwatch creates or stops a stopwatch for the given issue.
func IssueStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(issue, c.User) {
		c.NotFound("CanUseTimetracker", nil)
		return
	}

	if err := models.CreateOrStopIssueStopwatch(c.User, issue); err != nil {
		c.ServerError("CreateOrStopIssueStopwatch", err)
		return
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}

// CancelStopwatch cancel the stopwatch
func CancelStopwatch(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(issue, c.User) {
		c.NotFound("CanUseTimetracker", nil)
		return
	}

	if err := models.CancelStopwatch(c.User, issue); err != nil {
		c.ServerError("CancelStopwatch", err)
		return
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}
