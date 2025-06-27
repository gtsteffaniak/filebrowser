package access

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var accessCache = cache.NewCache(1 * time.Minute)

const accessRulesBucket = "access_rules"
const accessRulesKey = "rules"
const accessChangedKey = "newRule:"

// RuleSet groups users and groups for allow/deny lists.
type RuleSet struct {
	Users  map[string]struct{}
	Groups map[string]struct{}
}

// AccessRule defines allow/deny lists for a path.
type AccessRule struct {
	Deny  RuleSet
	Allow RuleSet
}

type FrontendRuleSet struct {
	Users  []string `json:"users"`
	Groups []string `json:"groups"`
}

type FrontendAccessRule struct {
	Deny  FrontendRuleSet `json:"deny"`
	Allow FrontendRuleSet `json:"allow"`
}

// GroupMap maps group names to a set of usernames.
type GroupMap map[string]map[string]struct{}

// Storage manages access rules and group membership.
type Storage struct {
	mux      sync.RWMutex
	AllRules map[string]map[string]*AccessRule // AllRules[sourcePath][indexPath]
	Groups   GroupMap                          // key: group name, value: set of usernames
	DB       *storm.DB                         // Optional: DB for persistence
	Users    *users.Storage                    // Reference to users storage
}

// SaveToDB persists all rules to the DB if DB is set.
func (s *Storage) SaveToDB() error {
	if s.DB == nil {
		return nil
	}
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

// NewStorage creates a new Storage instance. Optionally pass a DB for persistence and users storage.
// After creating Storage with a DB, call LoadFromDB() to load rules from the database on startup.
// Example:
//
//	store := NewStorage(db, usersStore)
//	err := store.LoadFromDB()
//	if err != nil { /* handle error */ }
func NewStorage(db *storm.DB, usersStore *users.Storage) *Storage {
	var s = &Storage{
		AllRules: make(map[string]map[string]*AccessRule),
		Groups:   make(GroupMap),
		DB:       db,
		Users:    usersStore,
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
			Deny:  RuleSet{Users: make(map[string]struct{}), Groups: make(map[string]struct{})},
			Allow: RuleSet{Users: make(map[string]struct{}), Groups: make(map[string]struct{})},
		}
		s.AllRules[sourcePath][indexPath] = rule
	} else {
		// Defensive: ensure maps are initialized
		if rule.Deny.Users == nil {
			rule.Deny.Users = make(map[string]struct{})
		}
		if rule.Deny.Groups == nil {
			rule.Deny.Groups = make(map[string]struct{})
		}
		if rule.Allow.Users == nil {
			rule.Allow.Users = make(map[string]struct{})
		}
		if rule.Allow.Groups == nil {
			rule.Allow.Groups = make(map[string]struct{})
		}
	}
	logger.Debugf("Created rule for source: %s and index: %s", sourcePath, indexPath)
	accessCache.Set(accessChangedKey+sourcePath, false)
	return rule
}

// DenyUser adds a user to the deny list for a given source and index path.
func (s *Storage) DenyUser(sourcePath, indexPath, username string) error {
	if s.Users != nil {
		_, err := s.Users.Get(username)
		if err != nil {
			return fmt.Errorf("user '%s' does not exist: %w", username, err)
		}
	}
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	defer s.mux.Unlock()
	rule.Deny.Users[username] = struct{}{}
	return s.SaveToDB()
}

// AllowUser adds a user to the allow list for a given source and index path.
func (s *Storage) AllowUser(sourcePath, indexPath, username string) error {
	if s.Users != nil {
		_, err := s.Users.Get(username)
		if err != nil {
			return fmt.Errorf("user '%s' does not exist: %w", username, err)
		}
	}
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	defer s.mux.Unlock()
	rule.Allow.Users[username] = struct{}{}
	return s.SaveToDB()
}

// DenyGroup adds a group to the deny list for a given source and index path.
func (s *Storage) DenyGroup(sourcePath, indexPath, groupname string) error {
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	defer s.mux.Unlock()
	rule.Deny.Groups[groupname] = struct{}{}
	return s.SaveToDB()
}

// AllowGroup adds a group to the allow list for a given source and index path.
func (s *Storage) AllowGroup(sourcePath, indexPath, groupname string) error {
	rule := s.getOrCreateRule(sourcePath, indexPath)
	s.mux.Lock()
	defer s.mux.Unlock()
	rule.Allow.Groups[groupname] = struct{}{}
	return s.SaveToDB()
}

// Permitted checks if a username is permitted for a given sourcePath and indexPath, recursively checking parent directories.
func (s *Storage) Permitted(sourcePath, indexPath, username string) bool {
	logger.Debugf("Checking if user: %s is permitted for source: %s and index: %s", username, sourcePath, indexPath)
	_, newRules := accessCache.Get(accessChangedKey + sourcePath).(map[string]FrontendAccessRule)
	val, ok := accessCache.Get(sourcePath + indexPath + username).(bool)
	if ok && !newRules {
		logger.Debugf("Returning cached rule for source: %s and index: %s for user: %s", sourcePath, indexPath, username)
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
	logger.Debug("Checking rule for source:", sourcePath, "and index:", indexPath, "for user:", username)
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
	// Check user deny
	if _, found := rule.Deny.Users[username]; found {
		return false
	}
	// Check group deny
	for group := range rule.Deny.Groups {
		if s.isUserInGroup(username, group) {
			return false
		}
	}
	// If any allow is present, user must be in at least one
	hasUserAllow := len(rule.Allow.Users) > 0
	hasGroupAllow := len(rule.Allow.Groups) > 0
	if hasUserAllow || hasGroupAllow {
		if hasUserAllow {
			if _, found := rule.Allow.Users[username]; found {
				return true
			}
		}
		if hasGroupAllow {
			for group := range rule.Allow.Groups {
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
func (s *Storage) GetFrontendRules(sourcePath, indexPath string) (FrontendAccessRule, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	frontendRules := FrontendAccessRule{
		Deny: FrontendRuleSet{
			Users:  make([]string, 0),
			Groups: make([]string, 0),
		},
		Allow: FrontendRuleSet{
			Users:  make([]string, 0),
			Groups: make([]string, 0),
		},
	}
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return frontendRules, false
	}
	rule, ok := rulesBySource[indexPath]
	if !ok || rule == nil {
		return frontendRules, false
	}
	// Convert AccessRule to FrontendAccessRule
	frontendRules.Deny.Users = utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Users)))
	frontendRules.Deny.Groups = utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Groups)))
	frontendRules.Allow.Users = utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Users)))
	frontendRules.Allow.Groups = utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Groups)))

	logger.Debugf("Returning frontend rules for source: %s and index: %s", sourcePath, indexPath)
	return frontendRules, ok
}

// GetAllRules returns all access rules as a map.
func (s *Storage) GetAllRules(sourcePath string) (map[string]FrontendAccessRule, error) {
	value, ok := accessCache.Get(accessChangedKey + sourcePath).(map[string]FrontendAccessRule)
	if ok {
		fmt.Println("Returning cached rules for source:", sourcePath)
		return value, nil
	}

	s.mux.RLock()
	defer s.mux.RUnlock()
	// Return a copy to avoid external mutation
	frontendRules := make(map[string]FrontendAccessRule, len(s.AllRules))
	rules, ok := s.AllRules[sourcePath]
	if !ok {
		return nil, fmt.Errorf("access: source not found: %s", sourcePath)
	}
	for indexPath, rule := range rules {
		// Convert AccessRule to FrontendAccessRule
		frontendRules[indexPath] = FrontendAccessRule{
			Deny: FrontendRuleSet{
				Users:  utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Users))),
				Groups: utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Groups))),
			},
			Allow: FrontendRuleSet{
				Users:  utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Users))),
				Groups: utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Groups))),
			},
		}
	}
	// cache responses
	accessCache.Set(accessChangedKey+sourcePath, frontendRules)
	return frontendRules, nil
}

// AddUserToGroup adds a username to a group.
func (s *Storage) AddUserToGroup(group, username string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if _, ok := s.Groups[group]; !ok {
		s.Groups[group] = make(map[string]struct{})
	}
	s.Groups[group][username] = struct{}{}
}

// RemoveUserFromGroup removes a username from a group.
func (s *Storage) RemoveUserFromGroup(group, username string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if users, ok := s.Groups[group]; ok {
		delete(users, username)
	}
}

// RemoveAllowUser removes a user from the allow list for a given source and index path.
func (s *Storage) RemoveAllowUser(sourcePath, indexPath, username string) (bool, error) {
	fmt.Println("Removing allow user:", username, "for source:", sourcePath, "and index:", indexPath)
	s.mux.Lock()
	defer s.mux.Unlock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		fmt.Println("Rule not found for source:", sourcePath, "and index:", indexPath)
		return false, nil
	}
	_, exists := rule.Allow.Users[username]
	if exists {
		delete(rule.Allow.Users, username)
	}
	removed := false
	if exists {
		logger.Debugf("Removing allow user: %s for source: %s and index: %s", username, sourcePath, indexPath)
		accessCache.Set(accessChangedKey+sourcePath, false)
		removed = true
	}
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], indexPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		return exists, s.SaveToDB()
	}
	return false, nil
}

// RemoveAllowGroup removes a group from the allow list for a given source and index path.
func (s *Storage) RemoveAllowGroup(sourcePath, indexPath, groupname string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Allow.Groups[groupname]
	if exists {
		delete(rule.Allow.Groups, groupname)
	}
	removed := false
	if exists {
		logger.Debugf("Removing allow group: %s for source: %s and index: %s", groupname, sourcePath, indexPath)
		accessCache.Set(accessChangedKey+sourcePath, false)
		removed = true
	}
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], indexPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveDenyUser removes a user from the deny list for a given source and index path.
func (s *Storage) RemoveDenyUser(sourcePath, indexPath, username string) (bool, error) {
	fmt.Println("Removing deny user:", username, "for source:", sourcePath, "and index:", indexPath)
	s.mux.Lock()
	defer s.mux.Unlock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Deny.Users[username]
	if exists {
		delete(rule.Deny.Users, username)
	}
	removed := false
	if exists {
		logger.Debugf("Removing deny user: %s for source: %s and index: %s", username, sourcePath, indexPath)
		accessCache.Set(accessChangedKey+sourcePath, false)
		removed = true
	}
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], indexPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		return exists, s.SaveToDB()
	}
	return false, nil
}

// RemoveDenyGroup removes a group from the deny list for a given source and index path.
func (s *Storage) RemoveDenyGroup(sourcePath, indexPath, groupname string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Deny.Groups[groupname]
	if exists {
		delete(rule.Deny.Groups, groupname)
	}
	removed := false
	if exists {
		logger.Debugf("Removing deny group: %s for source: %s and index: %s", groupname, sourcePath, indexPath)
		accessCache.Set(accessChangedKey+sourcePath, false)
		removed = true
	}
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], indexPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		return exists, s.SaveToDB()
	}
	return exists, nil
}
