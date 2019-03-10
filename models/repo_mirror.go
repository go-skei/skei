// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/git"
	"code.skei.dev/skei/modules/cache"
	"code.skei.dev/skei/modules/log"
	"code.skei.dev/skei/modules/process"
	"code.skei.dev/skei/modules/setting"
	"code.skei.dev/skei/modules/sync"
	"code.skei.dev/skei/modules/util"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	"gopkg.in/ini.v1"
)

// MirrorQueue holds an UniqueQueue object of the mirror
var MirrorQueue = sync.NewUniqueQueue(setting.Repository.MirrorQueueLength)

// Mirror represents mirror information of a repository.
type Mirror struct {
	ID          int64       `xorm:"pk autoincr"`
	RepoID      int64       `xorm:"INDEX"`
	Repo        *Repository `xorm:"-"`
	Interval    time.Duration
	EnablePrune bool `xorm:"NOT NULL DEFAULT true"`

	UpdatedUnix    util.TimeStamp `xorm:"INDEX"`
	NextUpdateUnix util.TimeStamp `xorm:"INDEX"`

	address string `xorm:"-"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (m *Mirror) BeforeInsert() {
	if m != nil {
		m.UpdatedUnix = util.TimeStampNow()
		m.NextUpdateUnix = util.TimeStampNow()
	}
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (m *Mirror) AfterLoad(session *xorm.Session) {
	if m == nil {
		return
	}

	var err error
	m.Repo, err = getRepositoryByID(session, m.RepoID)
	if err != nil {
		log.Error(3, "getRepositoryByID[%d]: %v", m.ID, err)
	}
}

// ScheduleNextUpdate calculates and sets next update time.
func (m *Mirror) ScheduleNextUpdate() {
	if m.Interval != 0 {
		m.NextUpdateUnix = util.TimeStampNow().AddDuration(m.Interval)
	} else {
		m.NextUpdateUnix = 0
	}
}

func remoteAddress(repoPath string) (string, error) {
	cfg, err := ini.Load(GitConfigPath(repoPath))
	if err != nil {
		return "", err
	}
	return cfg.Section("remote \"origin\"").Key("url").Value(), nil
}

func (m *Mirror) readAddress() {
	if len(m.address) > 0 {
		return
	}
	var err error
	m.address, err = remoteAddress(m.Repo.RepoPath())
	if err != nil {
		log.Error(4, "remoteAddress: %v", err)
	}
}

// sanitizeOutput sanitizes output of a command, replacing occurrences of the
// repository's remote address with a sanitized version.
func sanitizeOutput(output, repoPath string) (string, error) {
	remoteAddr, err := remoteAddress(repoPath)
	if err != nil {
		// if we're unable to load the remote address, then we're unable to
		// sanitize.
		return "", err
	}
	return util.SanitizeMessage(output, remoteAddr), nil
}

// Address returns mirror address from Git repository config without credentials.
func (m *Mirror) Address() string {
	m.readAddress()
	return util.SanitizeURLCredentials(m.address, false)
}

// FullAddress returns mirror address from Git repository config.
func (m *Mirror) FullAddress() string {
	m.readAddress()
	return m.address
}

// SaveAddress writes new address to Git repository config.
func (m *Mirror) SaveAddress(addr string) error {
	configPath := m.Repo.GitConfigPath()
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("Load: %v", err)
	}

	cfg.Section("remote \"origin\"").Key("url").SetValue(addr)
	return cfg.SaveToIndent(configPath, "\t")
}

// gitShortEmptySha Git short empty SHA
const gitShortEmptySha = "0000000"

// mirrorSyncResult contains information of a updated reference.
// If the oldCommitID is "0000000", it means a new reference, the value of newCommitID is empty.
// If the newCommitID is "0000000", it means the reference is deleted, the value of oldCommitID is empty.
type mirrorSyncResult struct {
	refName     string
	oldCommitID string
	newCommitID string
}

// parseRemoteUpdateOutput detects create, update and delete operations of references from upstream.
func parseRemoteUpdateOutput(output string) []*mirrorSyncResult {
	results := make([]*mirrorSyncResult, 0, 3)
	lines := strings.Split(output, "\n")
	for i := range lines {
		// Make sure reference name is presented before continue
		idx := strings.Index(lines[i], "-> ")
		if idx == -1 {
			continue
		}

		refName := lines[i][idx+3:]

		switch {
		case strings.HasPrefix(lines[i], " * "): // New reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " - "): // Delete reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				newCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], "   "): // New commits of a reference
			delimIdx := strings.Index(lines[i][3:], " ")
			if delimIdx == -1 {
				log.Error(2, "SHA delimiter not found: %q", lines[i])
				continue
			}
			shas := strings.Split(lines[i][3:delimIdx+3], "..")
			if len(shas) != 2 {
				log.Error(2, "Expect two SHAs but not what found: %q", lines[i])
				continue
			}
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: shas[0],
				newCommitID: shas[1],
			})

		default:
			log.Warn("parseRemoteUpdateOutput: unexpected update line %q", lines[i])
		}
	}
	return results
}

// runSync returns true if sync finished without error.
func (m *Mirror) runSync() ([]*mirrorSyncResult, bool) {
	repoPath := m.Repo.RepoPath()
	wikiPath := m.Repo.WikiPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	gitArgs := []string{"remote", "update"}
	if m.EnablePrune {
		gitArgs = append(gitArgs, "--prune")
	}

	_, stderr, err := process.GetManager().ExecDir(
		timeout, repoPath, fmt.Sprintf("Mirror.runSync: %s", repoPath),
		"git", gitArgs...)
	if err != nil {
		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		message, err := sanitizeOutput(stderr, repoPath)
		if err != nil {
			log.Error(4, "sanitizeOutput: %v", err)
			return nil, false
		}
		desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", repoPath, message)
		log.Error(4, desc)
		if err = CreateRepositoryNotice(desc); err != nil {
			log.Error(4, "CreateRepositoryNotice: %v", err)
		}
		return nil, false
	}
	output := stderr

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil, false
	}
	if err = SyncReleasesWithTags(m.Repo, gitRepo); err != nil {
		log.Error(4, "Failed to synchronize tags to releases for repository: %v", err)
	}

	if err := m.Repo.UpdateSize(); err != nil {
		log.Error(4, "Failed to update size for mirror repository: %v", err)
	}

	if m.Repo.HasWiki() {
		if _, stderr, err := process.GetManager().ExecDir(
			timeout, wikiPath, fmt.Sprintf("Mirror.runSync: %s", wikiPath),
			"git", "remote", "update", "--prune"); err != nil {
			// sanitize the output, since it may contain the remote address, which may
			// contain a password
			message, err := sanitizeOutput(stderr, wikiPath)
			if err != nil {
				log.Error(4, "sanitizeOutput: %v", err)
				return nil, false
			}
			desc := fmt.Sprintf("Failed to update mirror wiki repository '%s': %s", wikiPath, message)
			log.Error(4, desc)
			if err = CreateRepositoryNotice(desc); err != nil {
				log.Error(4, "CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}

	branches, err := m.Repo.GetBranches()
	if err != nil {
		log.Error(4, "GetBranches: %v", err)
		return nil, false
	}

	for i := range branches {
		cache.Remove(m.Repo.GetCommitsCountCacheKey(branches[i].Name, true))
	}

	m.UpdatedUnix = util.TimeStampNow()
	return parseRemoteUpdateOutput(output), true
}

func getMirrorByRepoID(e Engine, repoID int64) (*Mirror, error) {
	m := &Mirror{RepoID: repoID}
	has, err := e.Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMirrorNotExist
	}
	return m, nil
}

// GetMirrorByRepoID returns mirror information of a repository.
func GetMirrorByRepoID(repoID int64) (*Mirror, error) {
	return getMirrorByRepoID(x, repoID)
}

func updateMirror(e Engine, m *Mirror) error {
	_, err := e.ID(m.ID).AllCols().Update(m)
	return err
}

// UpdateMirror updates the mirror
func UpdateMirror(m *Mirror) error {
	return updateMirror(x, m)
}

// DeleteMirrorByRepoID deletes a mirror by repoID
func DeleteMirrorByRepoID(repoID int64) error {
	_, err := x.Delete(&Mirror{RepoID: repoID})
	return err
}

// MirrorUpdate checks and updates mirror repositories.
func MirrorUpdate() {
	if !taskStatusTable.StartIfNotRunning(mirrorUpdate) {
		return
	}
	defer taskStatusTable.Stop(mirrorUpdate)

	log.Trace("Doing: MirrorUpdate")

	if err := x.
		Where("next_update_unix<=?", time.Now().Unix()).
		And("next_update_unix!=0").
		Iterate(new(Mirror), func(idx int, bean interface{}) error {
			m := bean.(*Mirror)
			if m.Repo == nil {
				log.Error(4, "Disconnected mirror repository found: %d", m.ID)
				return nil
			}

			MirrorQueue.Add(m.RepoID)
			return nil
		}); err != nil {
		log.Error(4, "MirrorUpdate: %v", err)
	}
}

// SyncMirrors checks and syncs mirrors.
// TODO: sync more mirrors at same time.
func SyncMirrors() {
	sess := x.NewSession()
	defer sess.Close()
	// Start listening on new sync requests.
	for repoID := range MirrorQueue.Queue() {
		log.Trace("SyncMirrors [repo_id: %v]", repoID)
		MirrorQueue.Remove(repoID)

		m, err := GetMirrorByRepoID(com.StrTo(repoID).MustInt64())
		if err != nil {
			log.Error(4, "GetMirrorByRepoID [%s]: %v", repoID, err)
			continue
		}

		results, ok := m.runSync()
		if !ok {
			continue
		}

		m.ScheduleNextUpdate()
		if err = updateMirror(sess, m); err != nil {
			log.Error(4, "UpdateMirror [%s]: %v", repoID, err)
			continue
		}

		var gitRepo *git.Repository
		if len(results) == 0 {
			log.Trace("SyncMirrors [repo_id: %d]: no commits fetched", m.RepoID)
		} else {
			gitRepo, err = git.OpenRepository(m.Repo.RepoPath())
			if err != nil {
				log.Error(2, "OpenRepository [%d]: %v", m.RepoID, err)
				continue
			}
		}

		for _, result := range results {
			// Discard GitHub pull requests, i.e. refs/pull/*
			if strings.HasPrefix(result.refName, "refs/pull/") {
				continue
			}

			// Create reference
			if result.oldCommitID == gitShortEmptySha {
				if err = MirrorSyncCreateAction(m.Repo, result.refName); err != nil {
					log.Error(2, "MirrorSyncCreateAction [repo_id: %d]: %v", m.RepoID, err)
				}
				continue
			}

			// Delete reference
			if result.newCommitID == gitShortEmptySha {
				if err = MirrorSyncDeleteAction(m.Repo, result.refName); err != nil {
					log.Error(2, "MirrorSyncDeleteAction [repo_id: %d]: %v", m.RepoID, err)
				}
				continue
			}

			// Push commits
			oldCommitID, err := git.GetFullCommitID(gitRepo.Path, result.oldCommitID)
			if err != nil {
				log.Error(2, "GetFullCommitID [%d]: %v", m.RepoID, err)
				continue
			}
			newCommitID, err := git.GetFullCommitID(gitRepo.Path, result.newCommitID)
			if err != nil {
				log.Error(2, "GetFullCommitID [%d]: %v", m.RepoID, err)
				continue
			}
			commits, err := gitRepo.CommitsBetweenIDs(newCommitID, oldCommitID)
			if err != nil {
				log.Error(2, "CommitsBetweenIDs [repo_id: %d, new_commit_id: %s, old_commit_id: %s]: %v", m.RepoID, newCommitID, oldCommitID, err)
				continue
			}
			if err = MirrorSyncPushAction(m.Repo, MirrorSyncPushActionOptions{
				RefName:     result.refName,
				OldCommitID: oldCommitID,
				NewCommitID: newCommitID,
				Commits:     ListToPushCommits(commits),
			}); err != nil {
				log.Error(2, "MirrorSyncPushAction [repo_id: %d]: %v", m.RepoID, err)
				continue
			}
		}

		// Get latest commit date and update to current repository updated time
		commitDate, err := git.GetLatestCommitTime(m.Repo.RepoPath())
		if err != nil {
			log.Error(2, "GetLatestCommitDate [%s]: %v", m.RepoID, err)
			continue
		}

		if _, err = sess.Exec("UPDATE repository SET updated_unix = ? WHERE id = ?", commitDate.Unix(), m.RepoID); err != nil {
			log.Error(2, "Update repository 'updated_unix' [%s]: %v", m.RepoID, err)
			continue
		}
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	go SyncMirrors()
}
