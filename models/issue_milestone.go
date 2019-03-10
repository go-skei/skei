// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.skei.dev/skei/modules/log"
	"code.skei.dev/skei/modules/setting"
	"code.skei.dev/skei/modules/util"
	api "code.gitea.io/sdk/gitea"
	"github.com/go-xorm/xorm"
)

// Milestone represents a milestone of repository.
type Milestone struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	Name            string
	Content         string `xorm:"TEXT"`
	RenderedContent string `xorm:"-"`
	IsClosed        bool
	NumIssues       int
	NumClosedIssues int
	NumOpenIssues   int  `xorm:"-"`
	Completeness    int  // Percentage(1-100).
	IsOverdue       bool `xorm:"-"`

	DeadlineString string `xorm:"-"`
	DeadlineUnix   util.TimeStamp
	ClosedDateUnix util.TimeStamp

	TotalTrackedTime int64 `xorm:"-"`
}

// BeforeUpdate is invoked from XORM before updating this object.
func (m *Milestone) BeforeUpdate() {
	if m.NumIssues > 0 {
		m.Completeness = m.NumClosedIssues * 100 / m.NumIssues
	} else {
		m.Completeness = 0
	}
}

// AfterLoad is invoked from XORM after setting the value of a field of
// this object.
func (m *Milestone) AfterLoad() {
	m.NumOpenIssues = m.NumIssues - m.NumClosedIssues
	if m.DeadlineUnix.Year() == 9999 {
		return
	}

	m.DeadlineString = m.DeadlineUnix.Format("2006-01-02")
	if util.TimeStampNow() >= m.DeadlineUnix {
		m.IsOverdue = true
	}
}

// State returns string representation of milestone status.
func (m *Milestone) State() api.StateType {
	if m.IsClosed {
		return api.StateClosed
	}
	return api.StateOpen
}

// APIFormat returns this Milestone in API format.
func (m *Milestone) APIFormat() *api.Milestone {
	apiMilestone := &api.Milestone{
		ID:           m.ID,
		State:        m.State(),
		Title:        m.Name,
		Description:  m.Content,
		OpenIssues:   m.NumOpenIssues,
		ClosedIssues: m.NumClosedIssues,
	}
	if m.IsClosed {
		apiMilestone.Closed = m.ClosedDateUnix.AsTimePtr()
	}
	if m.DeadlineUnix.Year() < 9999 {
		apiMilestone.Deadline = m.DeadlineUnix.AsTimePtr()
	}
	return apiMilestone
}

// NewMilestone creates new milestone of repository.
func NewMilestone(m *Milestone) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(m); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `repository` SET num_milestones = num_milestones + 1 WHERE id = ?", m.RepoID); err != nil {
		return err
	}
	return sess.Commit()
}

func getMilestoneByRepoID(e Engine, repoID, id int64) (*Milestone, error) {
	m := &Milestone{
		ID:     id,
		RepoID: repoID,
	}
	has, err := e.Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMilestoneNotExist{id, repoID}
	}
	return m, nil
}

// GetMilestoneByRepoID returns the milestone in a repository.
func GetMilestoneByRepoID(repoID, id int64) (*Milestone, error) {
	return getMilestoneByRepoID(x, repoID, id)
}

// GetMilestoneByID returns the milestone via id .
func GetMilestoneByID(id int64) (*Milestone, error) {
	var m Milestone
	has, err := x.ID(id).Get(&m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMilestoneNotExist{id, 0}
	}
	return &m, nil
}

// MilestoneList is a list of milestones offering additional functionality
type MilestoneList []*Milestone

func (milestones MilestoneList) loadTotalTrackedTimes(e Engine) error {
	type totalTimesByMilestone struct {
		MilestoneID int64
		Time        int64
	}
	if len(milestones) == 0 {
		return nil
	}
	var trackedTimes = make(map[int64]int64, len(milestones))

	// Get total tracked time by milestone_id
	rows, err := e.Table("issue").
		Join("INNER", "milestone", "issue.milestone_id = milestone.id").
		Join("LEFT", "tracked_time", "tracked_time.issue_id = issue.id").
		Select("milestone_id, sum(time) as time").
		In("milestone_id", milestones.getMilestoneIDs()).
		GroupBy("milestone_id").
		Rows(new(totalTimesByMilestone))
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var totalTime totalTimesByMilestone
		err = rows.Scan(&totalTime)
		if err != nil {
			return err
		}
		trackedTimes[totalTime.MilestoneID] = totalTime.Time
	}

	for _, milestone := range milestones {
		milestone.TotalTrackedTime = trackedTimes[milestone.ID]
	}
	return nil
}

// LoadTotalTrackedTimes loads for every milestone in the list the TotalTrackedTime by a batch request
func (milestones MilestoneList) LoadTotalTrackedTimes() error {
	return milestones.loadTotalTrackedTimes(x)
}

func (milestones MilestoneList) getMilestoneIDs() []int64 {
	var ids = make([]int64, 0, len(milestones))
	for _, ms := range milestones {
		ids = append(ids, ms.ID)
	}
	return ids
}

// GetMilestonesByRepoID returns all opened milestones of a repository.
func GetMilestonesByRepoID(repoID int64) (MilestoneList, error) {
	miles := make([]*Milestone, 0, 10)
	return miles, x.Where("repo_id = ? AND is_closed = ?", repoID, false).
		Asc("deadline_unix").Asc("id").Find(&miles)
}

// GetMilestones returns a list of milestones of given repository and status.
func GetMilestones(repoID int64, page int, isClosed bool, sortType string) (MilestoneList, error) {
	miles := make([]*Milestone, 0, setting.UI.IssuePagingNum)
	sess := x.Where("repo_id = ? AND is_closed = ?", repoID, isClosed)
	if page > 0 {
		sess = sess.Limit(setting.UI.IssuePagingNum, (page-1)*setting.UI.IssuePagingNum)
	}

	switch sortType {
	case "furthestduedate":
		sess.Desc("deadline_unix")
	case "leastcomplete":
		sess.Asc("completeness")
	case "mostcomplete":
		sess.Desc("completeness")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("deadline_unix")
	}
	return miles, sess.Find(&miles)
}

func updateMilestone(e Engine, m *Milestone) error {
	_, err := e.ID(m.ID).AllCols().Update(m)
	return err
}

// UpdateMilestone updates information of given milestone.
func UpdateMilestone(m *Milestone) error {
	return updateMilestone(x, m)
}

func countRepoMilestones(e Engine, repoID int64) (int64, error) {
	return e.
		Where("repo_id=?", repoID).
		Count(new(Milestone))
}

func countRepoClosedMilestones(e Engine, repoID int64) (int64, error) {
	return e.
		Where("repo_id=? AND is_closed=?", repoID, true).
		Count(new(Milestone))
}

// CountRepoClosedMilestones returns number of closed milestones in given repository.
func CountRepoClosedMilestones(repoID int64) (int64, error) {
	return countRepoClosedMilestones(x, repoID)
}

// MilestoneStats returns number of open and closed milestones of given repository.
func MilestoneStats(repoID int64) (open int64, closed int64, err error) {
	open, err = x.
		Where("repo_id=? AND is_closed=?", repoID, false).
		Count(new(Milestone))
	if err != nil {
		return 0, 0, nil
	}
	closed, err = CountRepoClosedMilestones(repoID)
	return open, closed, err
}

// ChangeMilestoneStatus changes the milestone open/closed status.
func ChangeMilestoneStatus(m *Milestone, isClosed bool) (err error) {
	repo, err := GetRepositoryByID(m.RepoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	m.IsClosed = isClosed
	if err = updateMilestone(sess, m); err != nil {
		return err
	}

	numMilestones, err := countRepoMilestones(sess, repo.ID)
	if err != nil {
		return err
	}
	numClosedMilestones, err := countRepoClosedMilestones(sess, repo.ID)
	if err != nil {
		return err
	}
	repo.NumMilestones = int(numMilestones)
	repo.NumClosedMilestones = int(numClosedMilestones)

	if _, err = sess.ID(repo.ID).Cols("num_milestones, num_closed_milestones").Update(repo); err != nil {
		return err
	}
	return sess.Commit()
}

func changeMilestoneIssueStats(e *xorm.Session, issue *Issue) error {
	if issue.MilestoneID == 0 {
		return nil
	}

	m, err := getMilestoneByRepoID(e, issue.RepoID, issue.MilestoneID)
	if err != nil {
		return err
	}

	if issue.IsClosed {
		m.NumOpenIssues--
		m.NumClosedIssues++
	} else {
		m.NumOpenIssues++
		m.NumClosedIssues--
	}

	return updateMilestone(e, m)
}

func changeMilestoneAssign(e *xorm.Session, doer *User, issue *Issue, oldMilestoneID int64) error {
	if oldMilestoneID > 0 {
		m, err := getMilestoneByRepoID(e, issue.RepoID, oldMilestoneID)
		if err != nil {
			return err
		}

		m.NumIssues--
		if issue.IsClosed {
			m.NumClosedIssues--
		}

		if err = updateMilestone(e, m); err != nil {
			return err
		}
	}

	if issue.MilestoneID > 0 {
		m, err := getMilestoneByRepoID(e, issue.RepoID, issue.MilestoneID)
		if err != nil {
			return err
		}

		m.NumIssues++
		if issue.IsClosed {
			m.NumClosedIssues++
		}

		if err = updateMilestone(e, m); err != nil {
			return err
		}
	}

	if err := issue.loadRepo(e); err != nil {
		return err
	}

	if oldMilestoneID > 0 || issue.MilestoneID > 0 {
		if _, err := createMilestoneComment(e, doer, issue.Repo, issue, oldMilestoneID, issue.MilestoneID); err != nil {
			return err
		}
	}

	return updateIssueCols(e, issue, "milestone_id")
}

// ChangeMilestoneAssign changes assignment of milestone for issue.
func ChangeMilestoneAssign(issue *Issue, doer *User, oldMilestoneID int64) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = changeMilestoneAssign(sess, doer, issue, oldMilestoneID); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	var hookAction api.HookIssueAction
	if issue.MilestoneID > 0 {
		hookAction = api.HookIssueMilestoned
	} else {
		hookAction = api.HookIssueDemilestoned
	}

	if err = issue.LoadAttributes(); err != nil {
		return err
	}

	mode, _ := AccessLevel(doer, issue.Repo)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue()
		if err != nil {
			log.Error(2, "LoadIssue: %v", err)
			return
		}
		err = PrepareWebhooks(issue.Repo, HookEventPullRequest, &api.PullRequestPayload{
			Action:      hookAction,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		})
	} else {
		err = PrepareWebhooks(issue.Repo, HookEventIssues, &api.IssuePayload{
			Action:     hookAction,
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(2, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go HookQueue.Add(issue.RepoID)
	}
	return nil
}

// DeleteMilestoneByRepoID deletes a milestone from a repository.
func DeleteMilestoneByRepoID(repoID, id int64) error {
	m, err := GetMilestoneByRepoID(repoID, id)
	if err != nil {
		if IsErrMilestoneNotExist(err) {
			return nil
		}
		return err
	}

	repo, err := GetRepositoryByID(m.RepoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.ID(m.ID).Delete(new(Milestone)); err != nil {
		return err
	}

	numMilestones, err := countRepoMilestones(sess, repo.ID)
	if err != nil {
		return err
	}
	numClosedMilestones, err := countRepoClosedMilestones(sess, repo.ID)
	if err != nil {
		return err
	}
	repo.NumMilestones = int(numMilestones)
	repo.NumClosedMilestones = int(numClosedMilestones)

	if _, err = sess.ID(repo.ID).Cols("num_milestones, num_closed_milestones").Update(repo); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `issue` SET milestone_id = 0 WHERE milestone_id = ?", m.ID); err != nil {
		return err
	}
	return sess.Commit()
}
