// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	api "code.gitea.io/sdk/gitea"

	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/context"
	"code.skei.dev/skei/routers/api/v1/convert"
	"code.skei.dev/skei/routers/api/v1/user"
)

// ListTeams list all the teams of an organization
func ListTeams(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/teams organization orgListTeams
	// ---
	// summary: List an organization's teams
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"
	org := ctx.Org.Organization
	if err := org.GetTeams(); err != nil {
		ctx.Error(500, "GetTeams", err)
		return
	}

	apiTeams := make([]*api.Team, len(org.Teams))
	for i := range org.Teams {
		apiTeams[i] = convert.ToTeam(org.Teams[i])
	}
	ctx.JSON(200, apiTeams)
}

// ListUserTeams list all the teams a user belongs to
func ListUserTeams(ctx *context.APIContext) {
	// swagger:operation GET /user/teams user userListTeams
	// ---
	// summary: List all the teams a user belongs to
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"
	teams, err := models.GetUserTeams(ctx.User.ID)
	if err != nil {
		ctx.Error(500, "GetUserTeams", err)
		return
	}

	cache := make(map[int64]*api.Organization)
	apiTeams := make([]*api.Team, len(teams))
	for i := range teams {
		apiOrg, ok := cache[teams[i].OrgID]
		if !ok {
			org, err := models.GetUserByID(teams[i].OrgID)
			if err != nil {
				ctx.Error(500, "GetUserByID", err)
				return
			}
			apiOrg = convert.ToOrganization(org)
			cache[teams[i].OrgID] = apiOrg
		}
		apiTeams[i] = convert.ToTeam(teams[i])
		apiTeams[i].Organization = apiOrg
	}
	ctx.JSON(200, apiTeams)
}

// GetTeam api for get a team
func GetTeam(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id} organization orgGetTeam
	// ---
	// summary: Get a team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"
	ctx.JSON(200, convert.ToTeam(ctx.Org.Team))
}

// CreateTeam api for create a team
func CreateTeam(ctx *context.APIContext, form api.CreateTeamOption) {
	// swagger:operation POST /orgs/{org}/teams organization orgCreateTeam
	// ---
	// summary: Create a team
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateTeamOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Team"
	team := &models.Team{
		OrgID:       ctx.Org.Organization.ID,
		Name:        form.Name,
		Description: form.Description,
		Authorize:   models.ParseAccessMode(form.Permission),
	}

	unitTypes := models.FindUnitTypes(form.Units...)

	if team.Authorize < models.AccessModeOwner {
		var units = make([]*models.TeamUnit, 0, len(form.Units))
		for _, tp := range unitTypes {
			units = append(units, &models.TeamUnit{
				OrgID: ctx.Org.Organization.ID,
				Type:  tp,
			})
		}
		team.Units = units
	}

	if err := models.NewTeam(team); err != nil {
		if models.IsErrTeamAlreadyExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "NewTeam", err)
		}
		return
	}

	ctx.JSON(201, convert.ToTeam(team))
}

// EditTeam api for edit a team
func EditTeam(ctx *context.APIContext, form api.EditTeamOption) {
	// swagger:operation PATCH /teams/{id} organization orgEditTeam
	// ---
	// summary: Edit a team
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to edit
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditTeamOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"
	team := ctx.Org.Team
	team.Name = form.Name
	team.Description = form.Description
	team.Authorize = models.ParseAccessMode(form.Permission)
	unitTypes := models.FindUnitTypes(form.Units...)

	if team.Authorize < models.AccessModeOwner {
		var units = make([]*models.TeamUnit, 0, len(form.Units))
		for _, tp := range unitTypes {
			units = append(units, &models.TeamUnit{
				OrgID: ctx.Org.Organization.ID,
				Type:  tp,
			})
		}
		team.Units = units
	}

	if err := models.UpdateTeam(team, true); err != nil {
		ctx.Error(500, "EditTeam", err)
		return
	}
	ctx.JSON(200, convert.ToTeam(team))
}

// DeleteTeam api for delete a team
func DeleteTeam(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id} organization orgDeleteTeam
	// ---
	// summary: Delete a team
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: team deleted
	if err := models.DeleteTeam(ctx.Org.Team); err != nil {
		ctx.Error(500, "DeleteTeam", err)
		return
	}
	ctx.Status(204)
}

// GetTeamMembers api for get a team's members
func GetTeamMembers(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/members organization orgListTeamMembers
	// ---
	// summary: List a team's members
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"
	isMember, err := models.IsOrganizationMember(ctx.Org.Team.OrgID, ctx.User.ID)
	if err != nil {
		ctx.Error(500, "IsOrganizationMember", err)
		return
	} else if !isMember {
		ctx.Status(404)
		return
	}
	team := ctx.Org.Team
	if err := team.GetMembers(); err != nil {
		ctx.Error(500, "GetTeamMembers", err)
		return
	}
	members := make([]*api.User, len(team.Members))
	for i, member := range team.Members {
		members[i] = member.APIFormat()
	}
	ctx.JSON(200, members)
}

// GetTeamMember api for get a particular member of team
func GetTeamMember(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/members/{username} organization orgListTeamMember
	// ---
	// summary: List a particular member of team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the member to list
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/User"
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	ctx.JSON(200, u.APIFormat())
}

// AddTeamMember api for add a member to a team
func AddTeamMember(ctx *context.APIContext) {
	// swagger:operation PUT /teams/{id}/members/{username} organization orgAddTeamMember
	// ---
	// summary: Add a team member
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := ctx.Org.Team.AddMember(u.ID); err != nil {
		ctx.Error(500, "AddMember", err)
		return
	}
	ctx.Status(204)
}

// RemoveTeamMember api for remove one member from a team
func RemoveTeamMember(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id}/members/{username} organization orgRemoveTeamMember
	// ---
	// summary: Remove a team member
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to remove
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	if err := ctx.Org.Team.RemoveMember(u.ID); err != nil {
		ctx.Error(500, "RemoveMember", err)
		return
	}
	ctx.Status(204)
}

// GetTeamRepos api for get a team's repos
func GetTeamRepos(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/repos organization orgListTeamRepos
	// ---
	// summary: List a team's repos
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"
	team := ctx.Org.Team
	if err := team.GetRepositories(); err != nil {
		ctx.Error(500, "GetTeamRepos", err)
	}
	repos := make([]*api.Repository, len(team.Repos))
	for i, repo := range team.Repos {
		access, err := models.AccessLevel(ctx.User, repo)
		if err != nil {
			ctx.Error(500, "GetTeamRepos", err)
			return
		}
		repos[i] = repo.APIFormat(access)
	}
	ctx.JSON(200, repos)
}

// getRepositoryByParams get repository by a team's organization ID and repo name
func getRepositoryByParams(ctx *context.APIContext) *models.Repository {
	repo, err := models.GetRepositoryByName(ctx.Org.Team.OrgID, ctx.Params(":reponame"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetRepositoryByName", err)
		}
		return nil
	}
	return repo
}

// AddTeamRepository api for adding a repository to a team
func AddTeamRepository(ctx *context.APIContext) {
	// swagger:operation PUT /teams/{id}/repos/{org}/{repo} organization orgAddTeamRepository
	// ---
	// summary: Add a repository to a team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to add
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := models.AccessLevel(ctx.User, repo); err != nil {
		ctx.Error(500, "AccessLevel", err)
		return
	} else if access < models.AccessModeAdmin {
		ctx.Error(403, "", "Must have admin-level access to the repository")
		return
	}
	if err := ctx.Org.Team.AddRepository(repo); err != nil {
		ctx.Error(500, "AddRepository", err)
		return
	}
	ctx.Status(204)
}

// RemoveTeamRepository api for removing a repository from a team
func RemoveTeamRepository(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id}/repos/{org}/{repo} organization orgRemoveTeamRepository
	// ---
	// summary: Remove a repository from a team
	// description: This does not delete the repository, it only removes the
	//              repository from the team.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to remove
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to remove
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := models.AccessLevel(ctx.User, repo); err != nil {
		ctx.Error(500, "AccessLevel", err)
		return
	} else if access < models.AccessModeAdmin {
		ctx.Error(403, "", "Must have admin-level access to the repository")
		return
	}
	if err := ctx.Org.Team.RemoveRepository(repo.ID); err != nil {
		ctx.Error(500, "RemoveRepository", err)
		return
	}
	ctx.Status(204)
}
