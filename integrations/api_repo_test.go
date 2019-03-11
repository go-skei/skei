// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.skei.dev/skei/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserReposNotLogin(t *testing.T) {
	prepareTestEnv(t)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	req := NewRequestf(t, "GET", "/api/v1/users/%s/repos", user.Name)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiRepos []api.Repository
	DecodeJSON(t, resp, &apiRepos)
	expectedLen := models.GetCount(t, models.Repository{OwnerID: user.ID},
		models.Cond("is_private = ?", false))
	assert.Len(t, apiRepos, expectedLen)
	for _, repo := range apiRepos {
		assert.EqualValues(t, user.ID, repo.Owner.ID)
		assert.False(t, repo.Private)
	}
}

func TestAPISearchRepo(t *testing.T) {
	prepareTestEnv(t)
	const keyword = "test"

	req := NewRequestf(t, "GET", "/api/v1/repos/search?q=%s", keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	var body api.SearchResults
	DecodeJSON(t, resp, &body)
	assert.NotEmpty(t, body.Data)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword)
		assert.False(t, repo.Private)
	}

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 15}).(*models.User)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 16}).(*models.User)
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 18}).(*models.User)
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 20}).(*models.User)
	orgUser := models.AssertExistsAndLoadBean(t, &models.User{ID: 17}).(*models.User)

	// Map of expected results, where key is user for login
	type expectedResults map[*models.User]struct {
		count           int
		repoOwnerID     int64
		repoName        string
		includesPrivate bool
	}

	testCases := []struct {
		name, requestURL string
		expectedResults
	}{
		{name: "RepositoriesMax50", requestURL: "/api/v1/repos/search?limit=50", expectedResults: expectedResults{
			nil:   {count: 19},
			user:  {count: 19},
			user2: {count: 19}},
		},
		{name: "RepositoriesMax10", requestURL: "/api/v1/repos/search?limit=10", expectedResults: expectedResults{
			nil:   {count: 10},
			user:  {count: 10},
			user2: {count: 10}},
		},
		{name: "RepositoriesDefaultMax10", requestURL: "/api/v1/repos/search?default", expectedResults: expectedResults{
			nil:   {count: 10},
			user:  {count: 10},
			user2: {count: 10}},
		},
		{name: "RepositoriesByName", requestURL: fmt.Sprintf("/api/v1/repos/search?q=%s", "big_test_"), expectedResults: expectedResults{
			nil:   {count: 7, repoName: "big_test_"},
			user:  {count: 7, repoName: "big_test_"},
			user2: {count: 7, repoName: "big_test_"}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user.ID), expectedResults: expectedResults{
			nil:   {count: 4},
			user:  {count: 8, includesPrivate: true},
			user2: {count: 4}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser2", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user2.ID), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user2: {count: 2, includesPrivate: true}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser3", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user3.ID), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user2: {count: 1},
			user3: {count: 4, includesPrivate: true}},
		},
		{name: "RepositoriesOwnedByOrganization", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", orgUser.ID), expectedResults: expectedResults{
			nil:   {count: 1, repoOwnerID: orgUser.ID},
			user:  {count: 2, repoOwnerID: orgUser.ID, includesPrivate: true},
			user2: {count: 1, repoOwnerID: orgUser.ID}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser4", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user4.ID), expectedResults: expectedResults{
			nil:   {count: 3},
			user:  {count: 3},
			user4: {count: 6, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeSource", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "source"), expectedResults: expectedResults{
			nil:   {count: 0},
			user:  {count: 0},
			user4: {count: 0, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeFork", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "fork"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeFork/Exclusive", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s&exclusive=1", user4.ID, "fork"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeMirror", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "mirror"), expectedResults: expectedResults{
			nil:   {count: 2},
			user:  {count: 2},
			user4: {count: 4, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeMirror/Exclusive", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s&exclusive=1", user4.ID, "mirror"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeCollaborative", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "collaborative"), expectedResults: expectedResults{
			nil:   {count: 0},
			user:  {count: 0},
			user4: {count: 0, includesPrivate: true}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for userToLogin, expected := range testCase.expectedResults {
				var session *TestSession
				var testName string
				var userID int64
				var token string
				if userToLogin != nil && userToLogin.ID > 0 {
					testName = fmt.Sprintf("LoggedUser%d", userToLogin.ID)
					session = loginUser(t, userToLogin.Name)
					token = getTokenForLoggedInUser(t, session)
					userID = userToLogin.ID
				} else {
					testName = "AnonymousUser"
					session = emptyTestSession(t)
				}

				t.Run(testName, func(t *testing.T) {
					request := NewRequest(t, "GET", testCase.requestURL+"&token="+token)
					response := session.MakeRequest(t, request, http.StatusOK)

					var body api.SearchResults
					DecodeJSON(t, response, &body)

					assert.Len(t, body.Data, expected.count)
					for _, repo := range body.Data {
						r := getRepo(t, repo.ID)
						hasAccess, err := models.HasAccess(userID, r)
						assert.NoError(t, err)
						assert.True(t, hasAccess)

						assert.NotEmpty(t, repo.Name)

						if len(expected.repoName) > 0 {
							assert.Contains(t, repo.Name, expected.repoName)
						}

						if expected.repoOwnerID > 0 {
							assert.Equal(t, expected.repoOwnerID, repo.Owner.ID)
						}

						if !expected.includesPrivate {
							assert.False(t, repo.Private)
						}
					}
				})
			}
		})
	}
}

var repoCache = make(map[int64]*models.Repository)

func getRepo(t *testing.T, repoID int64) *models.Repository {
	if _, ok := repoCache[repoID]; !ok {
		repoCache[repoID] = models.AssertExistsAndLoadBean(t, &models.Repository{ID: repoID}).(*models.Repository)
	}
	return repoCache[repoID]
}

func TestAPIViewRepo(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)

	var repo api.Repository
	DecodeJSON(t, resp, &repo)
	assert.EqualValues(t, 1, repo.ID)
	assert.EqualValues(t, "repo1", repo.Name)
}

func TestAPIOrgRepos(t *testing.T) {
	prepareTestEnv(t)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 5}).(*models.User)
	// User3 is an Org. Check their repos.
	sourceOrg := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)

	expectedResults := map[*models.User]struct {
		count           int
		includesPrivate bool
	}{
		nil:   {count: 1},
		user:  {count: 2, includesPrivate: true},
		user2: {count: 3, includesPrivate: true},
		user3: {count: 1},
	}

	for userToLogin, expected := range expectedResults {
		var session *TestSession
		var testName string
		var token string
		if userToLogin != nil && userToLogin.ID > 0 {
			testName = fmt.Sprintf("LoggedUser%d", userToLogin.ID)
			session = loginUser(t, userToLogin.Name)
			token = getTokenForLoggedInUser(t, session)
		} else {
			testName = "AnonymousUser"
			session = emptyTestSession(t)
		}
		t.Run(testName, func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/orgs/%s/repos?token="+token, sourceOrg.Name)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var apiRepos []*api.Repository
			DecodeJSON(t, resp, &apiRepos)
			assert.Len(t, apiRepos, expected.count)
			for _, repo := range apiRepos {
				if !expected.includesPrivate {
					assert.False(t, repo.Private)
				}
			}
		})
	}
}

func TestAPIGetRepoByIDUnauthorized(t *testing.T) {
	prepareTestEnv(t)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repositories/2?token="+token)
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIRepoMigrate(t *testing.T) {
	testCases := []struct {
		ctxUserID, userID  int64
		cloneURL, repoName string
		expectedStatus     int
	}{
		{ctxUserID: 1, userID: 2, cloneURL: "https://github.com/go-gitea/git.git", repoName: "git-admin", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, userID: 2, cloneURL: "https://github.com/go-gitea/git.git", repoName: "git-own", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, userID: 1, cloneURL: "https://github.com/go-gitea/git.git", repoName: "git-bad", expectedStatus: http.StatusForbidden},
		{ctxUserID: 2, userID: 3, cloneURL: "https://github.com/go-gitea/git.git", repoName: "git-org", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, userID: 6, cloneURL: "https://github.com/go-gitea/git.git", repoName: "git-bad-org", expectedStatus: http.StatusForbidden},
	}

	prepareTestEnv(t)
	for _, testCase := range testCases {
		user := models.AssertExistsAndLoadBean(t, &models.User{ID: testCase.ctxUserID}).(*models.User)
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session)
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate?token="+token, &api.MigrateRepoOption{
			CloneAddr: testCase.cloneURL,
			UID:       int(testCase.userID),
			RepoName:  testCase.repoName,
		})
		session.MakeRequest(t, req, testCase.expectedStatus)
	}
}

func TestAPIOrgRepoCreate(t *testing.T) {
	testCases := []struct {
		ctxUserID         int64
		orgName, repoName string
		expectedStatus    int
	}{
		{ctxUserID: 1, orgName: "user3", repoName: "repo-admin", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, orgName: "user3", repoName: "repo-own", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, orgName: "user6", repoName: "repo-bad-org", expectedStatus: http.StatusForbidden},
	}

	prepareTestEnv(t)
	for _, testCase := range testCases {
		user := models.AssertExistsAndLoadBean(t, &models.User{ID: testCase.ctxUserID}).(*models.User)
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/org/%s/repos?token="+token, testCase.orgName), &api.CreateRepoOption{
			Name: testCase.repoName,
		})
		session.MakeRequest(t, req, testCase.expectedStatus)
	}
}
