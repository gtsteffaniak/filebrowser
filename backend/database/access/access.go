package access

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var accessCache = cache.NewCache(1 * time.Minute)

const accessRulesBucket = "access_rules"
const accessRulesKey = "rules"
const accessChangedKey = "newRule:"

type RuleMap map[string]*AccessRule
type SourceRuleMap map[string]RuleMap

type StringSet map[string]struct{}

type dbStorage struct {
	AllRules SourceRuleMap `json:"all_rules"`
	Groups   GroupMap      `json:"groups"`
}

// RuleSet groups users and groups for allow/deny lists.
type RuleSet struct {
	Users  StringSet
	Groups StringSet
}

// AccessRule defines allow/deny lists for a path.
type AccessRule struct {
	DenyAll bool `json:"denyAll,omitempty"`
	Deny    RuleSet
	Allow   RuleSet
}

type FrontendRuleSet struct {
	Users  []string `json:"users"`
	Groups []string `json:"groups"`
}

type FrontendAccessRule struct {
	DenyAll           bool            `json:"denyAll,omitempty"`
	Deny              FrontendRuleSet `json:"deny"`
	Allow             FrontendRuleSet `json:"allow"`
	SourceDenyDefault bool            `json:"sourceDenyDefault"`
}

// GroupMap maps group names to a set of usernames.
type GroupMap map[string]StringSet

// Storage manages access rules and group membership.
type Storage struct {
	mux      sync.RWMutex
	AllRules SourceRuleMap  // AllRules[sourcePath][indexPath] - in-memory authoritative state
	Groups   GroupMap       // key: group name, value: set of usernames - in-memory authoritative state
	DB       *storm.DB      // Optional: DB for persistence
	Users    *users.Storage // Reference to users storage
}

// SaveToDB persists all rules to the DB if DB is set.
func (s *Storage) SaveToDB() error {
	if s.DB == nil {
		return nil
	}
	data, err := json.Marshal(&dbStorage{
		AllRules: s.AllRules,
		Groups:   s.Groups,
	})
	if err != nil {
		return err
	}
	return s.DB.Set(accessRulesBucket, accessRulesKey, data)
}

// Flush persists the current in-memory state to the backing store.
// Call during graceful shutdown to ensure DB matches memory.
func (s *Storage) Flush() error {
	return s.SaveToDB()
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
	var storage dbStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return err
	}
	s.mux.Lock()
	s.AllRules = storage.AllRules
	if s.AllRules == nil {
		s.AllRules = make(SourceRuleMap)
	}
	s.Groups = storage.Groups
	if s.Groups == nil {
		s.Groups = make(GroupMap)
	}
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
		AllRules: make(SourceRuleMap),
		Groups:   make(GroupMap),
		DB:       db,
		Users:    usersStore,
	}
	return s
}

// ClearCache clears the access cache (useful for testing)
func ClearCache() {
	// Recreate the cache to clear it
	accessCache = cache.NewCache(1 * time.Minute)
}

// getOrCreateRuleNL ensures a rule exists for the given source and index path.
// The caller must hold the lock.
func (s *Storage) getOrCreateRuleNL(sourcePath, indexPath string) *AccessRule {
	if _, ok := s.AllRules[sourcePath]; !ok {
		s.AllRules[sourcePath] = make(RuleMap)
	}
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		rule = &AccessRule{
			Deny:  RuleSet{Users: make(StringSet), Groups: make(StringSet)},
			Allow: RuleSet{Users: make(StringSet), Groups: make(StringSet)},
		}
		s.AllRules[sourcePath][indexPath] = rule
	}
	logger.Debugf("Created rule for source: %s and index: %s", sourcePath, indexPath)
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
	s.mux.Lock()
	defer s.mux.Unlock()
	rule := s.getOrCreateRuleNL(sourcePath, indexPath)
	if _, ok := rule.Deny.Users[username]; ok {
		return errors.ErrExist
	}
	rule.Deny.Users[username] = struct{}{}
	s.incrementSourceVersion(sourcePath)
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
	s.mux.Lock()
	defer s.mux.Unlock()
	rule := s.getOrCreateRuleNL(sourcePath, indexPath)
	if _, ok := rule.Allow.Users[username]; ok {
		return errors.ErrExist
	}
	rule.Allow.Users[username] = struct{}{}
	s.incrementSourceVersion(sourcePath)
	return s.SaveToDB()
}

// DenyGroup adds a group to the deny list for a given source and index path.
func (s *Storage) DenyGroup(sourcePath, indexPath, groupname string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	_, ok := s.Groups[groupname]
	if !ok {
		return fmt.Errorf("group '%s' does not exist", groupname)
	}
	rule := s.getOrCreateRuleNL(sourcePath, indexPath)
	if _, ok := rule.Deny.Groups[groupname]; ok {
		return errors.ErrExist
	}
	rule.Deny.Groups[groupname] = struct{}{}
	s.incrementSourceVersion(sourcePath)
	return s.SaveToDB()
}

// AllowGroup adds a group to the allow list for a given source and index path.
func (s *Storage) AllowGroup(sourcePath, indexPath, groupname string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	_, ok := s.Groups[groupname]
	if !ok {
		return fmt.Errorf("group '%s' does not exist", groupname)
	}
	rule := s.getOrCreateRuleNL(sourcePath, indexPath)
	if _, ok := rule.Allow.Groups[groupname]; ok {
		return errors.ErrExist
	}
	rule.Allow.Groups[groupname] = struct{}{}
	s.incrementSourceVersion(sourcePath)
	return s.SaveToDB()
}

// DenyAll sets a rule to deny all access for a given source and index path.
func (s *Storage) DenyAll(sourcePath, indexPath string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	rule := s.getOrCreateRuleNL(sourcePath, indexPath)
	if rule.DenyAll {
		return errors.ErrExist
	}
	rule.DenyAll = true
	s.incrementSourceVersion(sourcePath)
	return s.SaveToDB()
}

// Permitted checks if a username is permitted for a given sourcePath and indexPath, recursively checking parent directories.
func (s *Storage) Permitted(sourcePath, indexPath, username string) bool {

	// Get current version for the sourcePath
	versionKey := "version:" + sourcePath
	version := 0
	if v, ok := accessCache.Get(versionKey).(int); ok {
		version = v
	}

	// Check cache with versioned key
	permKey := fmt.Sprintf("perm:%s:%d:%s:%s", sourcePath, version, indexPath, username)
	if p, ok := accessCache.Get(permKey).(bool); ok {
		return p
	}

	// Not in cache, compute, then cache it.
	result := s.computePermitted(sourcePath, indexPath, username)
	accessCache.Set(permKey, result)
	return result
}

func (s *Storage) computePermitted(sourcePath, indexPath, username string) bool {
	for {
		permitted, found := s.permittedAtExactPath(sourcePath, indexPath, username)
		if found {
			return permitted
		}
		indexPath = utils.GetParentDirectoryPath(indexPath)
		if indexPath == "" {
			break
		}
	}

	// No rules found anywhere in the path hierarchy
	// Check if the source has DenyByDefault configured (acts like a root-level denyAll rule)
	sourceInfo, ok := settings.Config.Server.SourceMap[sourcePath]
	if !ok {
		logger.Errorf("source %s not found in config during access check", sourcePath)
		return false
	}

	if sourceInfo.Config.DenyByDefault {
		return false
	}

	return true
}

// permittedAtExactPath checks if a rule exists at the given path and evaluates it if so.
func (s *Storage) permittedAtExactPath(sourcePath, indexPath, username string) (bool, bool) {
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
	return s.permittedRule(rule, username, sourcePath), true
}

// permittedRule contains the old Permitted logic, but operates on a rule and username only.
func (s *Storage) permittedRule(rule *AccessRule, username string, sourcePath string) bool {
	// Check user deny first - this always takes precedence
	if _, found := rule.Deny.Users[username]; found {
		return false
	}
	// Check group deny
	for group := range rule.Deny.Groups {
		if s.isUserInGroup(username, group) {
			return false
		}
	}

	// Check allow rules - these can override DenyAll
	hasUserAllow := len(rule.Allow.Users) > 0
	hasGroupAllow := len(rule.Allow.Groups) > 0
	if hasUserAllow || hasGroupAllow {
		// If user is in any allow list, permit them (overrides DenyAll)
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

		// User is not in any allow list
		// Check DenyByDefault to determine behavior
		sourceInfo, ok := settings.Config.Server.SourceMap[sourcePath]
		if ok && sourceInfo.Config.DenyByDefault {
			// DenyByDefault=true: allow lists are exclusive, deny users not on the list
			return false
		} else {
			// DenyByDefault=false: allow lists are additive, allow users not on the list
			return true
		}
	}

	// Check DenyAll - only applies if no allow rules exist
	if rule.DenyAll {
		return false
	}

	// No specific rules apply - check source DenyByDefault setting
	sourceInfo, ok := settings.Config.Server.SourceMap[sourcePath]
	if ok && sourceInfo.Config.DenyByDefault {
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

	// Get source configuration
	sourceDenyDefault := false
	sourceInfo, sourceExists := settings.Config.Server.SourceMap[sourcePath]
	if sourceExists {
		sourceDenyDefault = sourceInfo.Config.DenyByDefault
	}

	frontendRules := FrontendAccessRule{
		SourceDenyDefault: sourceDenyDefault,
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
	frontendRules.DenyAll = rule.DenyAll
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
		return value, nil
	}

	s.mux.RLock()
	defer s.mux.RUnlock()

	// Get source configuration
	sourceDenyDefault := false
	sourceInfo, sourceExists := settings.Config.Server.SourceMap[sourcePath]
	if sourceExists {
		sourceDenyDefault = sourceInfo.Config.DenyByDefault
	}

	// Return a copy to avoid external mutation
	frontendRules := make(map[string]FrontendAccessRule, len(s.AllRules))
	rules, ok := s.AllRules[sourcePath]
	if !ok {
		return frontendRules, nil
	}
	for indexPath, rule := range rules {
		// Convert AccessRule to FrontendAccessRule
		frontendRules[indexPath] = FrontendAccessRule{
			DenyAll:           rule.DenyAll,
			SourceDenyDefault: sourceDenyDefault,
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
func (s *Storage) AddUserToGroup(group, username string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if _, ok := s.Groups[group]; !ok {
		s.Groups[group] = make(StringSet)
	}
	if _, ok := s.Groups[group][username]; ok {
		return nil
	}
	s.Groups[group][username] = struct{}{}
	return s.SaveToDB()
}

// GetAllGroups returns all group names.
func (s *Storage) GetAllGroups() []string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	groups := make([]string, 0, len(s.Groups))
	for group := range s.Groups {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

// GetUserGroups returns all groups for a specific user.
func (s *Storage) GetUserGroups(username string) []string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var groups []string
	for group, users := range s.Groups {
		if _, ok := users[username]; ok {
			groups = append(groups, group)
		}
	}
	return utils.NonNilSlice(groups)
}

// SyncUserGroups updates a user's group memberships.
// It removes the user from groups not in the new list and adds them to new ones.
func (s *Storage) SyncUserGroups(username string, newGroups []string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	changed := false

	// Create a set of new groups for efficient lookup
	newGroupsSet := make(StringSet, len(newGroups))
	for _, g := range newGroups {
		newGroupsSet[g] = struct{}{}
	}

	// Iterate over all existing groups to find the user's current memberships
	for group, users := range s.Groups {
		_, userIsInGroup := users[username]
		_, groupIsInNewSet := newGroupsSet[group]

		// If user is in a group that is not in their new set of groups, remove them.
		if userIsInGroup && !groupIsInNewSet {
			delete(s.Groups[group], username)
			changed = true
		}
	}

	// Add user to new groups
	for group := range newGroupsSet {
		if _, ok := s.Groups[group]; !ok {
			s.Groups[group] = make(StringSet)
		}
		if _, ok := s.Groups[group][username]; !ok {
			s.Groups[group][username] = struct{}{}
			changed = true
		}
	}
	if changed {
		return s.SaveToDB()
	}
	return nil
}

// RemoveUserFromGroup removes a username from a group.
func (s *Storage) RemoveUserFromGroup(group, username string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	users, ok := s.Groups[group]
	if !ok {
		return nil
	}
	if _, ok := users[username]; !ok {
		return nil
	}
	delete(users, username)
	if len(s.Groups[group]) == 0 {
		delete(s.Groups, group)
	}
	return s.SaveToDB()
}

// RemoveAllowUser removes a user from the allow list for a given source and index path.
func (s *Storage) RemoveAllowUser(sourcePath, indexPath, username string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Allow.Users[username]
	if exists {
		delete(rule.Allow.Users, username)
	}
	removed := false
	if exists {
		s.incrementSourceVersion(sourcePath)
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
		accessCache.Set(accessChangedKey+sourcePath, false)
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
		s.incrementSourceVersion(sourcePath)
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
		accessCache.Set(accessChangedKey+sourcePath, false)
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveDenyUser removes a user from the deny list for a given source and index path.
func (s *Storage) RemoveDenyUser(sourcePath, indexPath, username string) (bool, error) {
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
		s.incrementSourceVersion(sourcePath)
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
		accessCache.Set(accessChangedKey+sourcePath, false)
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
		s.incrementSourceVersion(sourcePath)
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
		accessCache.Set(accessChangedKey+sourcePath, false)
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveDenyAll removes the deny all rule for a given source and index path.
func (s *Storage) RemoveDenyAll(sourcePath, indexPath string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	rule, ok := s.AllRules[sourcePath][indexPath]
	if !ok {
		return false, nil
	}
	removed := false
	if rule.DenyAll {
		rule.DenyAll = false
		s.incrementSourceVersion(sourcePath)
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
		accessCache.Set(accessChangedKey+sourcePath, false)
		return true, s.SaveToDB()
	}
	return false, nil
}

// RemoveAllRulesForUser removes a user from all allow and deny lists.
func (s *Storage) RemoveAllRulesForUser(username string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	changed := false
	changedSourcePaths := make(map[string]struct{})
	for sourcePath, rulesBySource := range s.AllRules {
		for indexPath, rule := range rulesBySource {
			if _, exists := rule.Allow.Users[username]; exists {
				delete(rule.Allow.Users, username)
				changedSourcePaths[sourcePath] = struct{}{}
				changed = true
			}
			if _, exists := rule.Deny.Users[username]; exists {
				delete(rule.Deny.Users, username)
				changedSourcePaths[sourcePath] = struct{}{}
				changed = true
			}
			if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
				delete(s.AllRules[sourcePath], indexPath)
				if len(s.AllRules[sourcePath]) == 0 {
					delete(s.AllRules, sourcePath)
				}
			}
		}
	}
	for sp := range changedSourcePaths {
		s.incrementSourceVersion(sp)
		accessCache.Set(accessChangedKey+sp, false)
	}
	if changed {
		return s.SaveToDB()
	}
	return nil
}

// RemoveAllRulesForGroup removes a group from all allow and deny lists.
func (s *Storage) RemoveAllRulesForGroup(groupname string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	changed := false
	changedSourcePaths := make(map[string]struct{})
	for sourcePath, rulesBySource := range s.AllRules {
		for indexPath, rule := range rulesBySource {
			if _, exists := rule.Allow.Groups[groupname]; exists {
				delete(rule.Allow.Groups, groupname)
				changedSourcePaths[sourcePath] = struct{}{}
				changed = true
			}
			if _, exists := rule.Deny.Groups[groupname]; exists {
				delete(rule.Deny.Groups, groupname)
				changedSourcePaths[sourcePath] = struct{}{}
				changed = true
			}
			if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
				delete(s.AllRules[sourcePath], indexPath)
				if len(s.AllRules[sourcePath]) == 0 {
					delete(s.AllRules, sourcePath)
				}
			}
		}
	}
	for sp := range changedSourcePaths {
		s.incrementSourceVersion(sp)
		accessCache.Set(accessChangedKey+sp, false)
	}
	if changed {
		return s.SaveToDB()
	}
	return nil
}

// GetRulesForUser returns all rules for a specific user for a given sourcePath.
func (s *Storage) GetRulesForUser(sourcePath, username string) map[string]FrontendAccessRule {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// Get source configuration
	sourceDenyDefault := false
	sourceInfo, sourceExists := settings.Config.Server.SourceMap[sourcePath]
	if sourceExists {
		sourceDenyDefault = sourceInfo.Config.DenyByDefault
	}

	userRules := make(map[string]FrontendAccessRule)
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return userRules
	}
	for indexPath, rule := range rulesBySource {
		userHasRule := false
		if _, ok := rule.Allow.Users[username]; ok {
			userHasRule = true
		}
		if !userHasRule {
			if _, ok := rule.Deny.Users[username]; ok {
				userHasRule = true
			}
		}
		if userHasRule {
			userRules[indexPath] = FrontendAccessRule{
				DenyAll:           rule.DenyAll,
				SourceDenyDefault: sourceDenyDefault,
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
	}
	return userRules
}

// GetRulesForGroup returns all rules for a specific group for a given sourcePath.
func (s *Storage) GetRulesForGroup(sourcePath, groupname string) map[string]FrontendAccessRule {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// Get source configuration
	sourceDenyDefault := false
	sourceInfo, sourceExists := settings.Config.Server.SourceMap[sourcePath]
	if sourceExists {
		sourceDenyDefault = sourceInfo.Config.DenyByDefault
	}

	groupRules := make(map[string]FrontendAccessRule)
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return groupRules
	}
	for indexPath, rule := range rulesBySource {
		groupHasRule := false
		if _, ok := rule.Allow.Groups[groupname]; ok {
			groupHasRule = true
		}
		if !groupHasRule {
			if _, ok := rule.Deny.Groups[groupname]; ok {
				groupHasRule = true
			}
		}
		if groupHasRule {
			groupRules[indexPath] = FrontendAccessRule{
				DenyAll:           rule.DenyAll,
				SourceDenyDefault: sourceDenyDefault,
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
	}
	return groupRules
}

// GetAllRulesByUsers returns a map of usernames to their rules for a given sourcePath.
func (s *Storage) GetAllRulesByUsers(sourcePath string) map[string]map[string]FrontendAccessRule {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// Get source configuration
	sourceDenyDefault := false
	sourceInfo, sourceExists := settings.Config.Server.SourceMap[sourcePath]
	if sourceExists {
		sourceDenyDefault = sourceInfo.Config.DenyByDefault
	}

	allUserRules := make(map[string]map[string]FrontendAccessRule)
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return allUserRules
	}
	for indexPath, rule := range rulesBySource {
		hasAllowUsers := len(rule.Allow.Users) > 0
		hasDenyUsers := len(rule.Deny.Users) > 0
		if !hasAllowUsers && !hasDenyUsers {
			continue
		}
		frontendRule := FrontendAccessRule{
			DenyAll:           rule.DenyAll,
			SourceDenyDefault: sourceDenyDefault,
			Deny: FrontendRuleSet{
				Users:  utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Users))),
				Groups: utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Groups))),
			},
			Allow: FrontendRuleSet{
				Users:  utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Users))),
				Groups: utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Groups))),
			},
		}
		for user := range rule.Allow.Users {
			if _, ok := allUserRules[user]; !ok {
				allUserRules[user] = make(map[string]FrontendAccessRule)
			}
			allUserRules[user][indexPath] = frontendRule
		}
		for user := range rule.Deny.Users {
			if _, ok := allUserRules[user]; !ok {
				allUserRules[user] = make(map[string]FrontendAccessRule)
			}
			allUserRules[user][indexPath] = frontendRule
		}
	}
	return allUserRules
}

// GetAllRulesByGroups returns a map of groupnames to their rules for a given sourcePath.
func (s *Storage) GetAllRulesByGroups(sourcePath string) map[string]map[string]FrontendAccessRule {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// Get source configuration
	sourceDenyDefault := false
	sourceInfo, sourceExists := settings.Config.Server.SourceMap[sourcePath]
	if sourceExists {
		sourceDenyDefault = sourceInfo.Config.DenyByDefault
	}

	allGroupRules := make(map[string]map[string]FrontendAccessRule)
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return allGroupRules
	}
	for indexPath, rule := range rulesBySource {
		hasAllowGroups := len(rule.Allow.Groups) > 0
		hasDenyGroups := len(rule.Deny.Groups) > 0
		if !hasAllowGroups && !hasDenyGroups {
			continue
		}
		frontendRule := FrontendAccessRule{
			DenyAll:           rule.DenyAll,
			SourceDenyDefault: sourceDenyDefault,
			Deny: FrontendRuleSet{
				Users:  utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Users))),
				Groups: utils.NonNilSlice(slices.Collect(maps.Keys(rule.Deny.Groups))),
			},
			Allow: FrontendRuleSet{
				Users:  utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Users))),
				Groups: utils.NonNilSlice(slices.Collect(maps.Keys(rule.Allow.Groups))),
			},
		}
		for group := range rule.Allow.Groups {
			if _, ok := allGroupRules[group]; !ok {
				allGroupRules[group] = make(map[string]FrontendAccessRule)
			}
			allGroupRules[group][indexPath] = frontendRule
		}
		for group := range rule.Deny.Groups {
			if _, ok := allGroupRules[group]; !ok {
				allGroupRules[group] = make(map[string]FrontendAccessRule)
			}
			allGroupRules[group][indexPath] = frontendRule
		}
	}
	return allGroupRules
}

// incrementSourceVersion increments the version of a sourcePath to invalidate caches.
// The caller MUST hold the mutex lock.
func (s *Storage) incrementSourceVersion(sourcePath string) {
	key := "version:" + sourcePath
	version := 0
	if v, ok := accessCache.Get(key).(int); ok {
		version = v
	}
	accessCache.Set(key, version+1)
}
