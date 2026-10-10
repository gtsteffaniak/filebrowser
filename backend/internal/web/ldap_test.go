package web

import (
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
)

// newLDAPEntry creates a minimal *ldap.Entry for testing (DN and optional objectClass).
func newLDAPEntry(dn string, objectClasses []string) *ldap.Entry {
	attrs := []*ldap.EntryAttribute{}
	if len(objectClasses) > 0 {
		attrs = append(attrs, &ldap.EntryAttribute{Name: "objectClass", Values: objectClasses})
	}
	return &ldap.Entry{DN: dn, Attributes: attrs}
}

func TestPickUserEntry(t *testing.T) {
	tests := []struct {
		name    string
		entries []*ldap.Entry
		wantNil bool
		wantDN  string // if wantNil is false, expected entry DN
	}{
		{
			name: "single user entry by objectClass",
			entries: []*ldap.Entry{
				newLDAPEntry("cn=alice,ou=users,dc=test", []string{"user", "organizationalPerson"}),
			},
			wantNil: false,
			wantDN:  "cn=alice,ou=users,dc=test",
		},
		{
			name: "single user entry by ou=users in DN",
			entries: []*ldap.Entry{
				newLDAPEntry("cn=bob,ou=users,dc=example,dc=com", []string{"group"}),
			},
			wantNil: false,
			wantDN:  "cn=bob,ou=users,dc=example,dc=com",
		},
		{
			name: "user and virtual group - picks user",
			entries: []*ldap.Entry{
				newLDAPEntry("cn=akadmin,ou=virtual-groups,dc=test", []string{"group"}),
				newLDAPEntry("cn=akadmin,ou=users,dc=test", []string{"user"}),
			},
			wantNil: false,
			wantDN:  "cn=akadmin,ou=users,dc=test",
		},
		{
			name: "two user entries - returns nil",
			entries: []*ldap.Entry{
				newLDAPEntry("cn=u1,ou=users,dc=test", []string{"user"}),
				newLDAPEntry("cn=u2,ou=users,dc=test", []string{"user"}),
			},
			wantNil: true,
		},
		{
			name: "only group entries - returns nil",
			entries: []*ldap.Entry{
				newLDAPEntry("cn=admins,ou=groups,dc=test", []string{"group"}),
				newLDAPEntry("cn=akadmin,ou=virtual-groups,dc=test", []string{"group"}),
			},
			wantNil: true,
		},
		{
			name:    "empty entries",
			entries: []*ldap.Entry{},
			wantNil: true,
		},
		{
			name: "objectClass case insensitive",
			entries: []*ldap.Entry{
				newLDAPEntry("cn=u,ou=users,dc=test", []string{"USER"}),
			},
			wantNil: false,
			wantDN:  "cn=u,ou=users,dc=test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickUserEntry(tt.entries)
			if tt.wantNil && got != nil {
				t.Errorf("pickUserEntry() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("pickUserEntry() = nil, want non-nil entry")
			}
		})
	}
}

func TestLdapGroupMatches(t *testing.T) {
	tests := []struct {
		name       string
		member     string
		configured string
		want       bool
	}{
		{
			name:       "exact DN match",
			member:     "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			configured: "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			want:       true,
		},
		{
			name:       "CN match",
			member:     "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			configured: "authentik Admins",
			want:       true,
		},
		{
			name:       "CN match case-insensitive",
			member:     "cn=it department,ou=groups,dc=example,dc=com",
			configured: "IT Department",
			want:       true,
		},
		{
			name:       "full DN match case-insensitive",
			member:     "CN=Employees,OU=Groups,DC=Example,DC=COM",
			configured: "cn=employees,ou=groups,dc=example,dc=com",
			want:       true,
		},
		{
			name:       "config full DN vs member CN-only",
			member:     "Employees",
			configured: "cn=Employees,ou=groups,dc=example,dc=com",
			want:       true,
		},
		{
			name:       "CN no match",
			member:     "cn=Other Group,ou=groups,dc=test",
			configured: "authentik Admins",
			want:       false,
		},
		{
			name:       "exact no match",
			member:     "cn=admins,ou=groups,dc=test",
			configured: "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			want:       false,
		},
		{
			name:       "same CN different parents rejected",
			member:     "cn=Admins,ou=Other,dc=example,dc=com",
			configured: "cn=Admins,ou=Privileged,dc=example,dc=com",
			want:       false,
		},
		{
			name:       "whitespace trimmed",
			member:     "  cn=admins,ou=groups,dc=test  ",
			configured: "cn=admins,ou=groups,dc=test",
			want:       true,
		},
		{
			name:       "invalid DN returns false",
			member:     "not-a-valid-dn",
			configured: "admins",
			want:       false,
		},
		{
			name:       "empty configured no match",
			member:     "cn=admins,ou=groups,dc=test",
			configured: "",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ldapGroupMatches(tt.member, tt.configured)
			if got != tt.want {
				t.Errorf("ldapGroupMatches(%q, %q) = %v, want %v", tt.member, tt.configured, got, tt.want)
			}
		})
	}
}
