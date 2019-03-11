// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/base"
	"code.skei.dev/skei/modules/context"
	"code.skei.dev/skei/modules/setting"
	"code.skei.dev/skei/routers"
)

const (
	tplOrgs base.TplName = "admin/org/list"
)

// Organizations show all the organizations
func Organizations(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.organizations")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminOrganizations"] = true

	routers.RenderUserSearch(ctx, &models.SearchUserOptions{
		Type:     models.UserTypeOrganization,
		PageSize: setting.UI.Admin.OrgPagingNum,
	}, tplOrgs)
}
