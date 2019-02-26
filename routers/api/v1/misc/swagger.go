// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"code.skei.dev/skei/modules/base"
	"code.skei.dev/skei/modules/context"
)

// tplSwagger swagger page template
const tplSwagger base.TplName = "swagger/ui"

// Swagger render swagger-ui page with v1 json
func Swagger(ctx *context.Context) {
	ctx.Data["APIJSONVersion"] = "v1"
	ctx.HTML(200, tplSwagger)
}
