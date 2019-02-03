// Copyright 2019 Katie Jones. All rights reserved.
// This source code is licensed under the GNU GPL version 2 or (at your option)
// 3. Details on licensing can be found in the LICENSE file.

// Package detaillevel provides customization functionality based on the detail
// level of the current user.
package detaillevel

import (
	"strings"

	"code.gitea.io/gitea/modules/context"

	"github.com/go-macaron/i18n"
	"gopkg.in/macaron.v1"
)

// TranslatorMiddleware is a middleware providing localization and customization
// of text output based on user's detail level.
func TranslatorMiddleware() macaron.Handler {
	return func(ctx *context.Context, l i18n.Locale) {
		// Add translator variable to context.
		dlt := Translator{l, ctx.User.DetailLevel}
		ctx.Data["dlt"] = dlt

		// Map dlt so it can be used as a service by other Handlers.
		ctx.Map(dlt)
	}
}

// Type Translator gives the detail level and locale to be used in translation.
type Translator struct {
	locale       i18n.Locale
	detail_level string
}

// Return translation based on detail level and locale.
func (t Translator) Tr(format string, args ...interface{}) string {
	// Try getting translation with '__(detail_level)' appended to the format.
	format_with_detail_level := format + "__" + t.detail_level
	text_with_detail_level := t.locale.Tr(format_with_detail_level, args...)

	// If the format with __(detail_level) appended is not found, it will return
	// the format string without the section. Calculate the format string without
	// the section here so we can compare them.
	var format_with_detail_level_without_section string

	idx := strings.IndexByte(format_with_detail_level, '.')
	if idx > 0 {
		format_with_detail_level_without_section = format_with_detail_level[idx+1:]
	} else {
		format_with_detail_level_without_section = format_with_detail_level
	}

	// If the format with the detail level is not found, it will return the format
	// string (without section) unchanged. If that is the case, try again with no
	// detail level appended.
	if text_with_detail_level != format_with_detail_level_without_section {
		return text_with_detail_level
	} else {
		return t.locale.Tr(format, args...)
	}
}
