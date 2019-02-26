package routers

import (
	"code.skei.dev/skei/modules/base"
	"code.skei.dev/skei/modules/context"
)

// tplSwaggerV1Json swagger v1 json template
const tplSwaggerV1Json base.TplName = "swagger/v1_json"

// SwaggerV1Json render swagger v1 json
func SwaggerV1Json(ctx *context.Context) {
	ctx.HTML(200, tplSwaggerV1Json)
}
