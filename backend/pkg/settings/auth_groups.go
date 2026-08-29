package settings

const (
	defaultGroupsClaim     = "groups"
	defaultLdapGroupsClaim = "memberOf"
)

// ResolveGroupsClaim returns the configured groups claim or the shared default when empty.
func ResolveGroupsClaim(claim string) string {
	if claim == "" {
		return defaultGroupsClaim
	}
	return claim
}

// ResolveLdapGroupsClaim returns the configured LDAP groups attribute or memberOf when empty.
func ResolveLdapGroupsClaim(claim string) string {
	if claim == "" {
		return defaultLdapGroupsClaim
	}
	return claim
}

// applyAuthCommonDefaults sets shared defaults for external auth methods that support group claims.
func applyAuthCommonDefaults(c *AuthCommon) {
	c.GroupsClaim = ResolveGroupsClaim(c.GroupsClaim)
}
