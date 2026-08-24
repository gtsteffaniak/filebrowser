package settings

const defaultGroupsClaim = "groups"

// applyAuthCommonDefaults sets shared defaults for external auth methods that support group claims.
func applyAuthCommonDefaults(c *AuthCommon) {
	if c.GroupsClaim == "" {
		c.GroupsClaim = defaultGroupsClaim
	}
}
