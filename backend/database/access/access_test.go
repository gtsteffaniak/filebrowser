package access

import (
	"testing"

	"github.com/asdine/storm/v3"
)

func TestPermitted_UserBlacklist(t *testing.T) {
	s := NewStorage()
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if err := s.BlacklistUser("mnt/storage", "/secret", "alice"); err != nil {
		t.Errorf("BlacklistUser failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/secret", "alice") {
		t.Error("alice should not be permitted (blacklisted)")
	}
	if !s.Permitted("mnt/storage", "/secret", "bob") {
		t.Error("bob should be permitted (not blacklisted)")
	}
}

func TestPermitted_UserWhitelist(t *testing.T) {
	s := NewStorage()
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if err := s.WhitelistUser("mnt/storage", "/vip", "bob"); err != nil {
		t.Errorf("WhitelistUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should be permitted (whitelisted)")
	}
	if s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should not be permitted (not whitelisted)")
	}
}

func TestPermitted_GroupBlacklist(t *testing.T) {
	s := NewStorage()
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	s.AddUserToGroup("admins", "alice")
	if err := s.BlacklistGroup("mnt/storage", "/admin", "admins"); err != nil {
		t.Errorf("BlacklistGroup failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/admin", "bob") == false {
		t.Error("bob should be permitted (not in blacklisted group)")
	}
	if s.Permitted("mnt/storage", "/admin", "alice") {
		t.Error("alice should not be permitted (in blacklisted group)")
	}
}

func TestPermitted_GroupWhitelist(t *testing.T) {
	s := NewStorage()
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	s.AddUserToGroup("vip", "bob")
	if err := s.WhitelistGroup("mnt/storage", "/vip", "vip"); err != nil {
		t.Errorf("WhitelistGroup failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should be permitted (in whitelisted group)")
	}
	if s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should not be permitted (not in whitelisted group)")
	}
}

func TestPermitted_NoRule(t *testing.T) {
	s := NewStorage()
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if !s.Permitted("mnt/storage", "/public", "anyone") {
		t.Error("anyone should be permitted if no rule exists")
	}
}

func TestPermitted_CombinedRules(t *testing.T) {
	s := NewStorage()
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	s.AddUserToGroup("vip", "bob")
	s.AddUserToGroup("admins", "alice")
	if err := s.BlacklistUser("mnt/storage", "/combo", "eve"); err != nil {
		t.Errorf("BlacklistUser failed: %v", err)
	}
	if err := s.WhitelistUser("mnt/storage", "/combo", "carol"); err != nil {
		t.Errorf("WhitelistUser failed: %v", err)
	}
	if err := s.BlacklistGroup("mnt/storage", "/combo", "admins"); err != nil {
		t.Errorf("BlacklistGroup failed: %v", err)
	}
	if err := s.WhitelistGroup("mnt/storage", "/combo", "vip"); err != nil {
		t.Errorf("WhitelistGroup failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/combo", "eve") {
		t.Error("eve should not be permitted (user blacklisted)")
	}
	if !s.Permitted("mnt/storage", "/combo", "carol") {
		t.Error("carol should be permitted (user whitelisted)")
	}
	if s.Permitted("mnt/storage", "/combo", "alice") {
		t.Error("alice should not be permitted (in group blacklist)")
	}
	if !s.Permitted("mnt/storage", "/combo", "bob") {
		t.Error("bob should be permitted (in group whitelist)")
	}
}
