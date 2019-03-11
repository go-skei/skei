// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.skei.dev/skei/modules/base"
	"code.skei.dev/skei/modules/log"
	"code.skei.dev/skei/modules/setting"
	"code.skei.dev/skei/modules/util"

	"github.com/Unknwon/com"
)

const (
	// ProtectedBranchRepoID protected Repo ID
	ProtectedBranchRepoID = "GITEA_REPO_ID"
)

// ProtectedBranch struct
type ProtectedBranch struct {
	ID                        int64  `xorm:"pk autoincr"`
	RepoID                    int64  `xorm:"UNIQUE(s)"`
	BranchName                string `xorm:"UNIQUE(s)"`
	CanPush                   bool   `xorm:"NOT NULL DEFAULT false"`
	EnableWhitelist           bool
	WhitelistUserIDs          []int64        `xorm:"JSON TEXT"`
	WhitelistTeamIDs          []int64        `xorm:"JSON TEXT"`
	EnableMergeWhitelist      bool           `xorm:"NOT NULL DEFAULT false"`
	MergeWhitelistUserIDs     []int64        `xorm:"JSON TEXT"`
	MergeWhitelistTeamIDs     []int64        `xorm:"JSON TEXT"`
	ApprovalsWhitelistUserIDs []int64        `xorm:"JSON TEXT"`
	ApprovalsWhitelistTeamIDs []int64        `xorm:"JSON TEXT"`
	RequiredApprovals         int64          `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix               util.TimeStamp `xorm:"created"`
	UpdatedUnix               util.TimeStamp `xorm:"updated"`
}

// IsProtected returns if the branch is protected
func (protectBranch *ProtectedBranch) IsProtected() bool {
	return protectBranch.ID > 0
}

// CanUserPush returns if some user could push to this protected branch
func (protectBranch *ProtectedBranch) CanUserPush(userID int64) bool {
	if !protectBranch.EnableWhitelist {
		return false
	}

	if base.Int64sContains(protectBranch.WhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.WhitelistTeamIDs) == 0 {
		return false
	}

	in, err := IsUserInTeams(userID, protectBranch.WhitelistTeamIDs)
	if err != nil {
		log.Error(1, "IsUserInTeams:", err)
		return false
	}
	return in
}

// CanUserMerge returns if some user could merge a pull request to this protected branch
func (protectBranch *ProtectedBranch) CanUserMerge(userID int64) bool {
	if !protectBranch.EnableMergeWhitelist {
		return true
	}

	if base.Int64sContains(protectBranch.MergeWhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.MergeWhitelistTeamIDs) == 0 {
		return false
	}

	in, err := IsUserInTeams(userID, protectBranch.MergeWhitelistTeamIDs)
	if err != nil {
		log.Error(1, "IsUserInTeams:", err)
		return false
	}
	return in
}

// HasEnoughApprovals returns true if pr has enough granted approvals.
func (protectBranch *ProtectedBranch) HasEnoughApprovals(pr *PullRequest) bool {
	if protectBranch.RequiredApprovals == 0 {
		return true
	}
	return protectBranch.GetGrantedApprovalsCount(pr) >= protectBranch.RequiredApprovals
}

// GetGrantedApprovalsCount returns the number of granted approvals for pr. A granted approval must be authored by a user in an approval whitelist.
func (protectBranch *ProtectedBranch) GetGrantedApprovalsCount(pr *PullRequest) int64 {
	reviews, err := GetReviewersByPullID(pr.Issue.ID)
	if err != nil {
		log.Error(1, "GetUniqueApprovalsByPullRequestID:", err)
		return 0
	}

	approvals := int64(0)
	userIDs := make([]int64, 0)
	for _, review := range reviews {
		if review.Type != ReviewTypeApprove {
			continue
		}
		if base.Int64sContains(protectBranch.ApprovalsWhitelistUserIDs, review.ID) {
			approvals++
			continue
		}
		userIDs = append(userIDs, review.ID)
	}
	approvalTeamCount, err := UsersInTeamsCount(userIDs, protectBranch.ApprovalsWhitelistTeamIDs)
	if err != nil {
		log.Error(1, "UsersInTeamsCount:", err)
		return 0
	}
	return approvalTeamCount + approvals
}

// GetProtectedBranchByRepoID getting protected branch by repo ID
func GetProtectedBranchByRepoID(RepoID int64) ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Where("repo_id = ?", RepoID).Desc("updated_unix").Find(&protectedBranches)
}

// GetProtectedBranchBy getting protected branch by ID/Name
func GetProtectedBranchBy(repoID int64, BranchName string) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{RepoID: repoID, BranchName: BranchName}
	has, err := x.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// GetProtectedBranchByID getting protected branch by ID
func GetProtectedBranchByID(id int64) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{ID: id}
	has, err := x.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// WhitelistOptions represent all sorts of whitelists used for protected branches
type WhitelistOptions struct {
	UserIDs []int64
	TeamIDs []int64

	MergeUserIDs []int64
	MergeTeamIDs []int64

	ApprovalsUserIDs []int64
	ApprovalsTeamIDs []int64
}

// UpdateProtectBranch saves branch protection options of repository.
// If ID is 0, it creates a new record. Otherwise, updates existing record.
// This function also performs check if whitelist user and team's IDs have been changed
// to avoid unnecessary whitelist delete and regenerate.
func UpdateProtectBranch(repo *Repository, protectBranch *ProtectedBranch, opts WhitelistOptions) (err error) {
	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}

	whitelist, err := updateUserWhitelist(repo, protectBranch.WhitelistUserIDs, opts.UserIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistUserIDs = whitelist

	whitelist, err = updateUserWhitelist(repo, protectBranch.MergeWhitelistUserIDs, opts.MergeUserIDs)
	if err != nil {
		return err
	}
	protectBranch.MergeWhitelistUserIDs = whitelist

	whitelist, err = updateUserWhitelist(repo, protectBranch.ApprovalsWhitelistUserIDs, opts.ApprovalsUserIDs)
	if err != nil {
		return err
	}
	protectBranch.ApprovalsWhitelistUserIDs = whitelist

	// if the repo is in an organization
	whitelist, err = updateTeamWhitelist(repo, protectBranch.WhitelistTeamIDs, opts.TeamIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(repo, protectBranch.MergeWhitelistTeamIDs, opts.MergeTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.MergeWhitelistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(repo, protectBranch.ApprovalsWhitelistTeamIDs, opts.ApprovalsTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.ApprovalsWhitelistTeamIDs = whitelist

	// Make sure protectBranch.ID is not 0 for whitelists
	if protectBranch.ID == 0 {
		if _, err = x.Insert(protectBranch); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
		return nil
	}

	if _, err = x.ID(protectBranch.ID).AllCols().Update(protectBranch); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

// GetProtectedBranches get all protected branches
func (repo *Repository) GetProtectedBranches() ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Find(&protectedBranches, &ProtectedBranch{RepoID: repo.ID})
}

// IsProtectedBranch checks if branch is protected
func (repo *Repository) IsProtectedBranch(branchName string, doer *User) (bool, error) {
	if doer == nil {
		return true, nil
	}

	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Exist(protectedBranch)
	if err != nil {
		return true, err
	}
	return has, nil
}

// IsProtectedBranchForPush checks if branch is protected for push
func (repo *Repository) IsProtectedBranchForPush(branchName string, doer *User) (bool, error) {
	if doer == nil {
		return true, nil
	}

	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Get(protectedBranch)
	if err != nil {
		return true, err
	} else if has {
		return !protectedBranch.CanUserPush(doer.ID), nil
	}

	return false, nil
}

// IsProtectedBranchForMerging checks if branch is protected for merging
func (repo *Repository) IsProtectedBranchForMerging(pr *PullRequest, branchName string, doer *User) (bool, error) {
	if doer == nil {
		return true, nil
	}

	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Get(protectedBranch)
	if err != nil {
		return true, err
	} else if has {
		return !protectedBranch.CanUserMerge(doer.ID) || !protectedBranch.HasEnoughApprovals(pr), nil
	}

	return false, nil
}

// updateUserWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have write access to the repo.
func updateUserWhitelist(repo *Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasUsersChanged {
		return currentWhitelist, nil
	}

	whitelist = make([]int64, 0, len(newWhitelist))
	for _, userID := range newWhitelist {
		user, err := GetUserByID(userID)
		if err != nil {
			return nil, fmt.Errorf("GetUserByID [user_id: %d, repo_id: %d]: %v", userID, repo.ID, err)
		}
		perm, err := GetUserRepoPermission(repo, user)
		if err != nil {
			return nil, fmt.Errorf("GetUserRepoPermission [user_id: %d, repo_id: %d]: %v", userID, repo.ID, err)
		}

		if !perm.CanWrite(UnitTypeCode) {
			continue // Drop invalid user ID
		}

		whitelist = append(whitelist, userID)
	}

	return
}

// updateTeamWhitelist checks whether the team whitelist changed and returns a whitelist with
// the teams from newWhitelist which have write access to the repo.
func updateTeamWhitelist(repo *Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasTeamsChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasTeamsChanged {
		return currentWhitelist, nil
	}

	teams, err := GetTeamsWithAccessToRepo(repo.OwnerID, repo.ID, AccessModeRead)
	if err != nil {
		return nil, fmt.Errorf("GetTeamsWithAccessToRepo [org_id: %d, repo_id: %d]: %v", repo.OwnerID, repo.ID, err)
	}

	whitelist = make([]int64, 0, len(teams))
	for i := range teams {
		if com.IsSliceContainsInt64(newWhitelist, teams[i].ID) {
			whitelist = append(whitelist, teams[i].ID)
		}
	}

	return
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func (repo *Repository) DeleteProtectedBranch(id int64) (err error) {
	protectedBranch := &ProtectedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(protectedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("delete protected branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// DeletedBranch struct
type DeletedBranch struct {
	ID          int64          `xorm:"pk autoincr"`
	RepoID      int64          `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string         `xorm:"UNIQUE(s) NOT NULL"`
	Commit      string         `xorm:"UNIQUE(s) NOT NULL"`
	DeletedByID int64          `xorm:"INDEX"`
	DeletedBy   *User          `xorm:"-"`
	DeletedUnix util.TimeStamp `xorm:"INDEX created"`
}

// AddDeletedBranch adds a deleted branch to the database
func (repo *Repository) AddDeletedBranch(branchName, commit string, deletedByID int64) error {
	deletedBranch := &DeletedBranch{
		RepoID:      repo.ID,
		Name:        branchName,
		Commit:      commit,
		DeletedByID: deletedByID,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.InsertOne(deletedBranch); err != nil {
		return err
	}

	return sess.Commit()
}

// GetDeletedBranches returns all the deleted branches
func (repo *Repository) GetDeletedBranches() ([]*DeletedBranch, error) {
	deletedBranches := make([]*DeletedBranch, 0)
	return deletedBranches, x.Where("repo_id = ?", repo.ID).Desc("deleted_unix").Find(&deletedBranches)
}

// GetDeletedBranchByID get a deleted branch by its ID
func (repo *Repository) GetDeletedBranchByID(ID int64) (*DeletedBranch, error) {
	deletedBranch := &DeletedBranch{ID: ID}
	has, err := x.Get(deletedBranch)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return deletedBranch, nil
}

// RemoveDeletedBranch removes a deleted branch from the database
func (repo *Repository) RemoveDeletedBranch(id int64) (err error) {
	deletedBranch := &DeletedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(deletedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("remove deleted branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// LoadUser loads the user that deleted the branch
// When there's no user found it returns a NewGhostUser
func (deletedBranch *DeletedBranch) LoadUser() {
	user, err := GetUserByID(deletedBranch.DeletedByID)
	if err != nil {
		user = NewGhostUser()
	}
	deletedBranch.DeletedBy = user
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches() {
	if !taskStatusTable.StartIfNotRunning(`deleted_branches_cleanup`) {
		return
	}
	defer taskStatusTable.Stop(`deleted_branches_cleanup`)

	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-setting.Cron.DeletedBranchesCleanup.OlderThan)
	_, err := x.Where("deleted_unix < ?", deleteBefore.Unix()).Delete(new(DeletedBranch))
	if err != nil {
		log.Error(4, "DeletedBranchesCleanup: %v", err)
	}
}
