package users

// SanitizeTokenPermissions returns only global permission flags suitable for API token metadata and JWT claims.
func SanitizeTokenPermissions(p Permissions) Permissions {
	return GlobalPermissionsOnly(p)
}

// IntersectGlobalPermissions returns global flags present on both owner and token caps.
func IntersectGlobalPermissions(owner, caps Permissions) Permissions {
	ownerGlobal := GlobalPermissionsOnly(owner)
	capsGlobal := GlobalPermissionsOnly(caps)
	return Permissions{
		Admin:    ownerGlobal.Admin && capsGlobal.Admin,
		Api:      ownerGlobal.Api && capsGlobal.Api,
		Share:    ownerGlobal.Share && capsGlobal.Share,
		Realtime: ownerGlobal.Realtime && capsGlobal.Realtime,
	}
}

// HasAnyGlobalPermission reports whether any global permission flag is set.
func HasAnyGlobalPermission(p Permissions) bool {
	g := GlobalPermissionsOnly(p)
	return g.Admin || g.Api || g.Share || g.Realtime
}

// IsMinimalApiToken reports whether the token inherits the owner's globals at auth time
// rather than applying stored global permission caps.
func IsMinimalApiToken(t AuthToken) bool {
	if t.Minimal != nil {
		return *t.Minimal
	}
	// Legacy tokens lack minimal; empty stored global caps implied minimal (WebDAV-style).
	return !HasAnyGlobalPermission(t.Permissions)
}
