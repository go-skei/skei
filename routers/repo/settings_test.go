// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"testing"

	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/auth"
	"code.skei.dev/skei/modules/context"
	"code.skei.dev/skei/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestAddReadOnlyDeployKey(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/settings/keys")

	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)

	addKeyForm := auth.AddKeyForm{
		Title:   "read-only",
		Content: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
	}
	DeployKeysPost(ctx, addKeyForm)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    models.AccessModeRead,
	})
}

func TestAddReadWriteOnlyDeployKey(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/settings/keys")

	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)

	addKeyForm := auth.AddKeyForm{
		Title:      "read-write",
		Content:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		IsWritable: true,
	}
	DeployKeysPost(ctx, addKeyForm)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    models.AccessModeWrite,
	})
}

func TestCollaborationPost(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadUser(t, ctx, 4)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user4")

	u := &models.User{
		LowerName: "user2",
		Type:      models.UserTypeIndividual,
	}

	re := &models.Repository{
		ID:    2,
		Owner: u,
	}

	repo := &context.Repository{
		Owner:      u,
		Repository: re,
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	exists, err := re.IsCollaborator(4)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestCollaborationPost_InactiveUser(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadUser(t, ctx, 9)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user9")

	repo := &context.Repository{
		Owner: &models.User{
			LowerName: "user2",
		},
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestCollaborationPost_AddCollaboratorTwice(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadUser(t, ctx, 4)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user4")

	u := &models.User{
		LowerName: "user2",
		Type:      models.UserTypeIndividual,
	}

	re := &models.Repository{
		ID:    2,
		Owner: u,
	}

	repo := &context.Repository{
		Owner:      u,
		Repository: re,
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	exists, err := re.IsCollaborator(4)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Try adding the same collaborator again
	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestCollaborationPost_NonExistentUser(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user34")

	repo := &context.Repository{
		Owner: &models.User{
			LowerName: "user2",
		},
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}
