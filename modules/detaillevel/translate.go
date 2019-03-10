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
		dlt := Translator{ctx, l}
		ctx.Data["dlt"] = dlt

		// Map dlt so it can be used as a service by other Handlers.
		ctx.Map(dlt)
	}
}

// Type Translator gives the detail level and locale to be used in translation.
type Translator struct {
	ctx    *context.Context
	locale i18n.Locale
}

// Return translation based on detail level and locale.
func (t Translator) Tr(format string, args ...interface{}) string {
	// Get detail level from User, or use default.
	var detail_level string
	if t.ctx.User == nil {
		detail_level = "default"
	} else {
		detail_level = t.ctx.User.DetailLevel
	}

	// Get section and key from format string.
	var section_with_dot, key string
	idx := strings.IndexByte(format, '.')
	if idx > 0 {
		section_with_dot = format[:idx+1]
		key = format[idx+1:]
	} else {
		section_with_dot = ""
		key = format
	}

	// Check if translation with '__(detail_level)' appended to the key exists.
	// Have to call Tr without args, because then it will return the key unchanged
	// if the translation doesn't exist.
	key_with_dl := key + "__" + detail_level
	translation_with_dl_exists := (key_with_dl != t.locale.Tr(section_with_dot+key_with_dl))

	if translation_with_dl_exists {
		return t.locale.Tr(section_with_dot+key_with_dl, args...)
	} else {
		return t.locale.Tr(section_with_dot+key, args...)
	}
}
