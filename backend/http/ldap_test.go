package http

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
			if tt.wantNil {
				if got != nil {
					t.Errorf("pickUserEntry() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("pickUserEntry() = nil, want non-nil entry")
			}
			if got.DN != tt.wantDN {
				t.Errorf("pickUserEntry() DN = %v, want %v", got.DN, tt.wantDN)
			}
		})
	}
}

func TestLdapGroupMatchesAdmin(t *testing.T) {
	tests := []struct {
		name       string
		groupDN    string
		adminGroup string
		want       bool
	}{
		{
			name:       "exact DN match",
			groupDN:    "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			adminGroup: "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			want:       true,
		},
		{
			name:       "CN match",
			groupDN:    "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			adminGroup: "authentik Admins",
			want:       true,
		},
		{
			name:       "CN no match",
			groupDN:    "cn=Other Group,ou=groups,dc=test",
			adminGroup: "authentik Admins",
			want:       false,
		},
		{
			name:       "exact no match",
			groupDN:    "cn=admins,ou=groups,dc=test",
			adminGroup: "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			want:       false,
		},
		{
			name:       "whitespace trimmed",
			groupDN:    "  cn=admins,ou=groups,dc=test  ",
			adminGroup: "cn=admins,ou=groups,dc=test",
			want:       true,
		},
		{
			name:       "invalid DN returns false",
			groupDN:    "not-a-valid-dn",
			adminGroup: "admins",
			want:       false,
		},
		{
			name:       "empty adminGroup no match",
			groupDN:    "cn=admins,ou=groups,dc=test",
			adminGroup: "",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ldapGroupMatchesAdmin(tt.groupDN, tt.adminGroup)
			if got != tt.want {
				t.Errorf("ldapGroupMatchesAdmin(%q, %q) = %v, want %v", tt.groupDN, tt.adminGroup, got, tt.want)
			}
		})
	}
}
