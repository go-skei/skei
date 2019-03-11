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

func TestAPIAdminCreateAndDeleteSSHKey(t *testing.T) {
	prepareTestEnv(t)
	// user1 is an admin user
	session := loginUser(t, "user1")
	keyOwner := models.AssertExistsAndLoadBean(t, &models.User{Name: "user2"}).(*models.User)

	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys?token=%s", keyOwner.Name, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		"title": "test-key",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)
	models.AssertExistsAndLoadBean(t, &models.PublicKey{
		ID:          newPublicKey.ID,
		Name:        newPublicKey.Title,
		Content:     newPublicKey.Key,
		Fingerprint: newPublicKey.Fingerprint,
		OwnerID:     keyOwner.ID,
	})

	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d?token=%s",
		keyOwner.Name, newPublicKey.ID, token)
	session.MakeRequest(t, req, http.StatusNoContent)
	models.AssertNotExistsBean(t, &models.PublicKey{ID: newPublicKey.ID})
}

func TestAPIAdminDeleteMissingSSHKey(t *testing.T) {
	prepareTestEnv(t)
	// user1 is an admin user
	session := loginUser(t, "user1")

	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "DELETE", "/api/v1/admin/users/user1/keys/%d?token=%s", models.NonexistentID, token)
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIAdminDeleteUnauthorizedKey(t *testing.T) {
	prepareTestEnv(t)
	adminUsername := "user1"
	normalUsername := "user2"
	session := loginUser(t, adminUsername)

	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys?token=%s", adminUsername, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		"title": "test-key",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)

	session = loginUser(t, normalUsername)
	token = getTokenForLoggedInUser(t, session)
	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d?token=%s",
		adminUsername, newPublicKey.ID, token)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPISudoUser(t *testing.T) {
	prepareTestEnv(t)
	adminUsername := "user1"
	normalUsername := "user2"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/user?sudo=%s&token=%s", normalUsername, token)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var user api.User
	DecodeJSON(t, resp, &user)

	assert.Equal(t, normalUsername, user.UserName)
}

func TestAPISudoUserForbidden(t *testing.T) {
	prepareTestEnv(t)
	adminUsername := "user1"
	normalUsername := "user2"

	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/user?sudo=%s&token=%s", adminUsername, token)
	req := NewRequest(t, "GET", urlStr)
	session.MakeRequest(t, req, http.StatusForbidden)
}
