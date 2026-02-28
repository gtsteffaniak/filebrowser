package http

import (
	"crypto/tls"
	"fmt"
	"strings"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// AuthenticateLDAPUser attempts LDAP authentication and returns the filebrowser user if successful.
func AuthenticateLDAPUser(username, password string) (*users.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username required")
	}
	groups, userAttributes, err := authenticateLDAP(username, password)
	if err != nil {
		logger.Debugf("ldap authentication failed: %v", err)
		return nil, err
	}
	
	// Use the configured UserIdentifier if available, otherwise use the login username
	ldapCfg := settings.Config.Auth.Methods.LdapAuth
	mappedUsername := username
	if ldapCfg.UserIdentifier != "" {
		if val, ok := userAttributes[ldapCfg.UserIdentifier]; ok && val != "" {
			mappedUsername = val
			logger.Debugf("ldap: mapped username from %s: %s -> %s", ldapCfg.UserIdentifier, username, mappedUsername)
		} else {
			logger.Warningf("ldap: userIdentifier '%s' not found in LDAP entry for user %s, using login username", ldapCfg.UserIdentifier, username)
		}
	}
	
	logger.Debugf("ldap authentication successful, getting or creating user %s", mappedUsername)
	return getOrCreateLdapUser(mappedUsername, groups)
}

func authenticateLDAP(username, password string) ([]string, map[string]string, error) {
	c := settings.Config.Auth.Methods.LdapAuth
	logger.Debugf("ldap: connecting to %s", c.Server)

	var opts []ldap.DialOpt
	if c.DisableVerifyTLS && c.Scheme == "ldaps" {
		opts = append(opts, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
		logger.Warning("LDAP TLS verification is disabled.")
	}

	conn, err := ldap.DialURL(c.Server, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	// Bind with service account
	if c.UserPassword != "" {
		logger.Debugf("ldap: binding as service account %s", c.UserDN)
		if err = conn.Bind(c.UserDN, c.UserPassword); err != nil {
			return nil, nil, fmt.Errorf("ldap bind (service): %w", err)
		}
	} else {
		logger.Debugf("ldap: no service account bind (userPassword empty)")
	}

	// Build list of attributes to fetch
	groupAttr := c.GroupsClaim
	if groupAttr == "" {
		groupAttr = "memberOf"
	}
	attributes := []string{"dn", groupAttr}
	if c.UserIdentifier != "" {
		attributes = append(attributes, c.UserIdentifier)
	}

	filter := fmt.Sprintf(c.UserFilter, ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		c.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		attributes,
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("ldap search (%s): %w", c.Server, err)
	}

	if len(result.Entries) == 0 {
		return nil, nil, fmt.Errorf("user not found: %s (LDAP search for %s returned no entries)", username, filter)
	}

	entry := result.Entries[0]
	if len(result.Entries) > 1 {
		// Prefer the user entry when multiple match (e.g. Authentik returns user + virtual group with same cn).
		if u := pickUserEntry(result.Entries); u != nil {
			entry = u
			logger.Debugf("ldap: multiple entries, using user entry DN=%s", entry.DN)
		} else {
			return nil, nil, fmt.Errorf("multiple entries for user: %s (set userFilter to narrow, e.g. (&(cn=%%s)(objectClass=user)))", username)
		}
	}
	userDN := entry.DN

	// Verify password by binding as the user
	if err := conn.Bind(userDN, password); err != nil {
		logger.Debugf("ldap: bind as user failed: %v", err)
		return nil, nil, fmt.Errorf("ldap bind (user): %w", err)
	}

	// Re-bind as service account for any follow-up (we're done; this is optional)
	if c.UserPassword != "" {
		_ = conn.Bind(c.UserDN, c.UserPassword)
	}

	groups := entry.GetAttributeValues(groupAttr)
	
	// Extract user attributes for identifier mapping
	userAttributes := make(map[string]string)
	if c.UserIdentifier != "" {
		values := entry.GetAttributeValues(c.UserIdentifier)
		if len(values) > 0 {
			userAttributes[c.UserIdentifier] = values[0]
		}
	}
	
	return groups, userAttributes, nil
}

// pickUserEntry returns the entry that represents a user when multiple entries match (e.g. Authentik user + virtual group).
// Prefers entries with objectClass=user or DN under ou=users; returns nil if none or multiple user entries.
func pickUserEntry(entries []*ldap.Entry) *ldap.Entry {
	var user *ldap.Entry
	for _, e := range entries {
		oc := e.GetAttributeValues("objectClass")
		dnLower := strings.ToLower(e.DN)
		isUser := false
		for _, c := range oc {
			if strings.EqualFold(c, "user") {
				isUser = true
				break
			}
		}
		if !isUser && strings.Contains(dnLower, "ou=users") {
			isUser = true
		}
		if isUser {
			if user != nil {
				return nil // multiple user entries
			}
			user = e
		}
	}
	return user
}

// ldapGroupMatchesAdmin returns true if the LDAP group DN matches the configured admin group (full DN or CN value).
func ldapGroupMatchesAdmin(groupDN, adminGroup string) bool {
	g := strings.TrimSpace(groupDN)
	if g == adminGroup {
		return true
	}
	dn, err := ldap.ParseDN(g)
	if err != nil {
		return false
	}
	for _, rdn := range dn.RDNs {
		for _, attr := range rdn.Attributes {
			if strings.EqualFold(attr.Type, "cn") && attr.Value == adminGroup {
				return true
			}
		}
	}
	return false
}

// getOrCreateLdapUser returns the filebrowser user for an LDAP-authenticated username, creating one if configured.
func getOrCreateLdapUser(username string, groups []string) (*users.User, error) {
	logger.Debugf("getting or creating ldap user %s", username)
	ldapCfg := config.Auth.Methods.LdapAuth
	
	// Check if user is in required groups (if userGroups is configured)
	if len(ldapCfg.UserGroups) > 0 {
		userInAllowedGroup := false
		for _, allowedGroup := range ldapCfg.UserGroups {
			for _, userGroup := range groups {
				if ldapGroupMatchesAdmin(userGroup, allowedGroup) {
					userInAllowedGroup = true
					break
				}
			}
			if userInAllowedGroup {
				break
			}
		}
		if !userInAllowedGroup {
			logger.Warningf("User %s is not in any of the required groups %v. Access denied.", username, ldapCfg.UserGroups)
			return nil, fmt.Errorf("user %s is not authorized to access this application (not in required groups)", username)
		}
		logger.Debugf("User %s is in required group, allowing access.", username)
	}
	
	isAdmin := false
	if ldapCfg.AdminGroup != "" {
		for _, g := range groups {
			if ldapGroupMatchesAdmin(g, ldapCfg.AdminGroup) {
				isAdmin = true
				break
			}
		}
		if isAdmin {
			logger.Debugf("User %s is in admin group %s, granting admin privileges.", username, ldapCfg.AdminGroup)
		}
	}

	user, err := store.Users.Get(username)
	if err != nil {
		if err.Error() != "the resource does not exist" {
			return nil, err
		}
		// Auto-create user on first LDAP authentication
		if ldapCfg.AdminGroup == "" {
			isAdmin = config.UserDefaults.Permissions.Admin
		}
		user = &users.User{
			Username:    username,
			LoginMethod: users.LoginMethodLdap,
		}
		settings.ApplyUserDefaults(user)
		if isAdmin {
			user.Permissions.Admin = true
		}
		if err = storage.CreateUser(*user, user.Permissions); err != nil {
			return nil, err
		}
		user, err = store.Users.Get(username)
		if err != nil {
			return nil, err
		}
	} else {
		if user.LoginMethod != users.LoginMethodLdap {
			return nil, errors.ErrWrongLoginMethod
		}
		if ldapCfg.AdminGroup != "" && isAdmin != user.Permissions.Admin {
			user.Permissions.Admin = isAdmin
			_ = store.Users.Update(user, true, "Permissions")
		}
	}

	if err := store.Access.SyncUserGroups(username, groups); err != nil {
		logger.Warningf("failed to sync ldap user %s groups: %v", username, err)
	}
	return user, nil
}
