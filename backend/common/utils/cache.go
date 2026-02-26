package utils

import (
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
)

var (
	DiskUsageCache     = cache.NewCache[bool](30*time.Second, 24*time.Hour)
	RealPathCache      = cache.NewCache[string](48*time.Hour, 72*time.Hour)
	SearchResultsCache = cache.NewCache[string](15*time.Second, 1*time.Hour)
	OnlyOfficeCache    = cache.NewCache[string](48*time.Hour, 1*time.Hour)
	JwtCache           = cache.NewCache[string](1*time.Hour, 72*time.Hour)
	SSOTokenCache      = cache.NewCache[bool](2 * time.Minute) // 2 minute upstream login check for LDAP and OIDC. So if a user logs out of the OIDC/LDAP provider, their filebrowser session will expire within 2 minutes
)
