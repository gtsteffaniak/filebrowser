package access

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/go-cache/cache"
)

var accessCache = cache.NewCache(1 * time.Minute)

const accessRulesBucket = "access_rules"
const accessRulesKey = "rules"

// RuleSet groups users and groups for allow/deny lists.
type RuleSet struct {
	Users  map[string]struct{}
	Groups map[string]struct{}
}

// AccessRule defines allow/deny lists for a path.
type AccessRule struct {
	SourcePath  string `json:"source_path"`
	IndexPath   string `json:"index_path"`
	Blacklisted RuleSet
	Whitelisted RuleSet
}

// GroupMap maps group names to a set of usernames.
type GroupMap map[string]map[string]struct{}

// Storage manages access rules and group membership.
type Storage struct {
	mux      sync.RWMutex
	AllRules map[string]map[string]*AccessRule // AllRules[sourcePath][indexPath]
	Groups   GroupMap                          // key: group name, value: set of usernames
	DB       *storm.DB                         // Optional: DB for persistence
}

// SaveToDB persists all rules to the DB if DB is set.
func (s *Storage) SaveToDB() error {
	if s.DB == nil {
		return nil
	}
	s.mux.RLock()
	defer s.mux.RUnlock()
	data, err := json.Marshal(s.AllRules)
	if err != nil {
		return err
	}
	return s.DB.Set(accessRulesBucket, accessRulesKey, data)
}

// LoadFromDB loads all rules from the DB if DB is set.
func (s *Storage) LoadFromDB() error {
	if s.DB == nil {
		return nil
	}
	var data []byte
	err := s.DB.Get(accessRulesBucket, accessRulesKey, &data)
	if err != nil {
		return err
	}
	var rules map[string]map[string]*AccessRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}
	s.mux.Lock()
	s.AllRules = rules
	s.mux.Unlock()
	return nil
}

// NewStorage creates a new Storage instance. Optionally pass a DB for persistence.
// After creating Storage with a DB, call LoadFromDB() to load rules from the database on startup.
// Example:
//
//	store := NewStorage(db)
//	err := store.LoadFromDB()
//	if err != nil { /* handle error */ }
func NewStorage(db ...*storm.DB) *Storage {
	var s = &Storage{
		AllRules: make(map[string]map[string]*AccessRule),
		Groups:   make(GroupMap),
	}
	if len(db) > 0 {
		s.DB = db[0]
	}
	return s
}

// getOrCreateRule ensures a rule exists for the given source and index path.
func (s *Storage) getOrCreateRule(sourcePath, indexPath string) *AccessRule {
	s.mux.Lock()
	defer s.mux.Unlock()
	if _, ok := s.AllRules[sourcePath]; !ok {
		s.AllRules[sourcePath] = make(map[string]*AccessRule)
	}
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		rule = &AccessRule{
			SourcePath:  sourcePath,
			IndexPath:   indexPath,
			Blacklisted: RuleSet{Users: make(map[string]struct{}), Groups: make(map[string]struct{})},
			Whitelisted: RuleSet{Users: make(map[string]struct{}), Groups: make(map[string]struct{})},
		}
		s.AllRules[sourcePath][indexPath] = rule
	}
	return rule
}

// BlacklistUser adds a user to the blacklist for a given source and index path.
func (s *Storage) BlacklistUser(sourcePath, indexPath, username string) error {
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	rule.Blacklisted.Users[username] = struct{}{}
	s.mux.Unlock()
	return s.SaveToDB()
}

// WhitelistUser adds a user to the whitelist for a given source and index path.
func (s *Storage) WhitelistUser(sourcePath, indexPath, username string) error {
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	rule.Whitelisted.Users[username] = struct{}{}
	s.mux.Unlock()
	return s.SaveToDB()
}

// BlacklistGroup adds a group to the blacklist for a given source and index path.
func (s *Storage) BlacklistGroup(sourcePath, indexPath, groupname string) error {
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	rule.Blacklisted.Groups[groupname] = struct{}{}
	s.mux.Unlock()
	return s.SaveToDB()
}

// WhitelistGroup adds a group to the whitelist for a given source and index path.
func (s *Storage) WhitelistGroup(sourcePath, indexPath, groupname string) error {
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	rule.Whitelisted.Groups[groupname] = struct{}{}
	s.mux.Unlock()
	return s.SaveToDB()
}

// Permitted checks if a username is permitted for a given sourcePath and indexPath, recursively checking parent directories.
func (s *Storage) Permitted(sourcePath, indexPath, username string) bool {
	val, ok := accessCache.Get(sourcePath + indexPath + username).(bool)
	if ok {
		return val
	}
	for {
		permitted, found := s.permittedAtExactPath(sourcePath, indexPath, username)
		if found {
			accessCache.Set(sourcePath+indexPath+username, permitted)
			return permitted
		}
		indexPath = utils.GetParentDirectoryPath(indexPath)
		if indexPath == "" {
			break
		}
	}
	accessCache.Set(sourcePath+indexPath+username, true)
	return true
}

// permittedAtExactPath checks if a rule exists at the given path and evaluates it if so.
func (s *Storage) permittedAtExactPath(sourcePath, indexPath, username string) (bool, bool) {
	fmt.Println("Checking rule for source:", sourcePath, "and index:", indexPath, "for user:", username)
	s.mux.RLock()
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		s.mux.RUnlock()
		return true, false
	}
	rule, ok := rulesBySource[indexPath]
	s.mux.RUnlock()
	if !ok {
		return true, false
	}
	return s.permittedRule(rule, username), true
}

// permittedRule contains the old Permitted logic, but operates on a rule and username only.
func (s *Storage) permittedRule(rule *AccessRule, username string) bool {
	// Check user blacklist
	if _, found := rule.Blacklisted.Users[username]; found {
		return false
	}
	// Check group blacklist
	for group := range rule.Blacklisted.Groups {
		if s.isUserInGroup(username, group) {
			return false
		}
	}
	// If any whitelist is present, user must be in at least one
	hasUserWhitelist := len(rule.Whitelisted.Users) > 0
	hasGroupWhitelist := len(rule.Whitelisted.Groups) > 0
	if hasUserWhitelist || hasGroupWhitelist {
		if hasUserWhitelist {
			if _, found := rule.Whitelisted.Users[username]; found {
				return true
			}
		}
		if hasGroupWhitelist {
			for group := range rule.Whitelisted.Groups {
				if s.isUserInGroup(username, group) {
					return true
				}
			}
		}
		return false
	}
	return true
}

// isUserInGroup checks if a username is in a group.
func (s *Storage) isUserInGroup(username, group string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	users, ok := s.Groups[group]
	if !ok {
		return false
	}
	_, found := users[username]
	return found
}

// GetRule retrieves a rule for a sourcePath and indexPath.
func (s *Storage) GetRule(sourcePath, indexPath string) (AccessRule, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return AccessRule{}, false
	}
	rule, ok := rulesBySource[indexPath]
	return *rule, ok
}

// GetAllRules returns all access rules as a map.
func (s *Storage) GetAllRules() map[string]map[string]AccessRule {
	s.mux.RLock()
	defer s.mux.RUnlock()
	// Return a copy to avoid external mutation
	copy := make(map[string]map[string]AccessRule, len(s.AllRules))
	for source, rulesBySource := range s.AllRules {
		copy[source] = make(map[string]AccessRule, len(rulesBySource))
		for index, rule := range rulesBySource {
			copy[source][index] = *rule
		}
	}
	return copy
}

// AddUserToGroup adds a username to a group.
func (s *Storage) AddUserToGroup(group, username string) {
	s.mux.Lock()
	if _, ok := s.Groups[group]; !ok {
		s.Groups[group] = make(map[string]struct{})
	}
	s.Groups[group][username] = struct{}{}
	s.mux.Unlock()
}

// RemoveUserFromGroup removes a username from a group.
func (s *Storage) RemoveUserFromGroup(group, username string) {
	s.mux.Lock()
	if users, ok := s.Groups[group]; ok {
		delete(users, username)
	}
	s.mux.Unlock()
}

// RemoveWhitelistUser removes a user from the whitelist for a given source and index path.
func (s *Storage) RemoveWhitelistUser(sourcePath, indexPath, username string) (bool, error) {
	s.mux.Lock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		s.mux.Unlock()
		return false, nil
	}
	_, exists := rule.Whitelisted.Users[username]
	if exists {
		delete(rule.Whitelisted.Users, username)
	}
	s.mux.Unlock()
	if exists {
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveWhitelistGroup removes a group from the whitelist for a given source and index path.
func (s *Storage) RemoveWhitelistGroup(sourcePath, indexPath, groupname string) (bool, error) {
	s.mux.Lock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		s.mux.Unlock()
		return false, nil
	}
	_, exists := rule.Whitelisted.Groups[groupname]
	if exists {
		delete(rule.Whitelisted.Groups, groupname)
	}
	s.mux.Unlock()
	if exists {
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveBlacklistUser removes a user from the blacklist for a given source and index path.
func (s *Storage) RemoveBlacklistUser(sourcePath, indexPath, username string) (bool, error) {
	s.mux.Lock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		s.mux.Unlock()
		return false, nil
	}
	_, exists := rule.Blacklisted.Users[username]
	if exists {
		delete(rule.Blacklisted.Users, username)
	}
	s.mux.Unlock()
	if exists {
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveBlacklistGroup removes a group from the blacklist for a given source and index path.
func (s *Storage) RemoveBlacklistGroup(sourcePath, indexPath, groupname string) (bool, error) {
	s.mux.Lock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		s.mux.Unlock()
		return false, nil
	}
	_, exists := rule.Blacklisted.Groups[groupname]
	if exists {
		delete(rule.Blacklisted.Groups, groupname)
	}
	s.mux.Unlock()
	if exists {
		return exists, s.SaveToDB()
	}
	return exists, nil
}
