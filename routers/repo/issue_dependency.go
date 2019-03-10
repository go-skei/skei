// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/context"
)

// AddDependency adds new dependencies
func AddDependency(ctx *context.Context) {
	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx.User) {
		ctx.Error(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.QueryInt64("newDependency")

	issueIndex := ctx.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Redirect
	defer ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issueIndex), http.StatusSeeOther)

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_issue_not_exist"))
		return
	}

	// Check if both issues are in the same repo
	if issue.RepoID != dep.RepoID {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_same_repo"))
		return
	}

	// Check if issue and dependency is the same
	if dep.Index == issueIndex {
		ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_same_issue"))
		return
	}

	err = models.CreateIssueDependency(ctx.User, issue, dep)
	if err != nil {
		if models.IsErrDependencyExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_exists"))
			return
		} else if models.IsErrCircularDependency(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_cannot_create_circular"))
			return
		} else {
			ctx.ServerError("CreateOrUpdateIssueDependency", err)
			return
		}
	}
}

// RemoveDependency removes the dependency
func RemoveDependency(ctx *context.Context) {
	// Check if the Repo is allowed to have dependencies
	if !ctx.Repo.CanCreateIssueDependencies(ctx.User) {
		ctx.Error(http.StatusForbidden, "CanCreateIssueDependencies")
		return
	}

	depID := ctx.QueryInt64("removeDependencyID")

	issueIndex := ctx.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, issueIndex)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	// Redirect
	ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issueIndex), http.StatusSeeOther)

	// Dependency Type
	depTypeStr := ctx.Req.PostForm.Get("dependencyType")

	var depType models.DependencyType

	switch depTypeStr {
	case "blockedBy":
		depType = models.DependencyTypeBlockedBy
	case "blocking":
		depType = models.DependencyTypeBlocking
	default:
		ctx.Error(http.StatusBadRequest, "GetDependecyType")
		return
	}

	// Dependency
	dep, err := models.GetIssueByID(depID)
	if err != nil {
		ctx.ServerError("GetIssueByID", err)
		return
	}

	if err = models.RemoveIssueDependency(ctx.User, issue, dep, depType); err != nil {
		if models.IsErrDependencyNotExists(err) {
			ctx.Flash.Error(ctx.Tr("repo.issues.dependency.add_error_dep_not_exist"))
			return
		}
		ctx.ServerError("RemoveIssueDependency", err)
		return
	}
}
