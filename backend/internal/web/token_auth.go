package web

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// applyNamedApiTokenGlobalCaps intersects owner globals with JWT global caps for named custom API tokens.
// Session WEB_TOKEN_* tokens and minimal API tokens (no global caps in claims) keep full owner globals.
func applyNamedApiTokenGlobalCaps(user *users.User, tk users.AuthToken, tokenName string) {
	if user == nil {
		return
	}
	if strings.HasPrefix(tokenName, "WEB_TOKEN") {
		return
	}
	if tk.BelongsTo == 0 || !users.HasAnyGlobalPermission(tk.Permissions) {
		return
	}
	user.Permissions = users.IntersectGlobalPermissions(user.Permissions, tk.Permissions)
}
