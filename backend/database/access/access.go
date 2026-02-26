package access

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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

var (
	accessCache     = cache.NewCache[string](1 * time.Minute)                        // for accessChangedKey
	versionCache    = cache.NewCache[int](1 * time.Minute)                           // for version keys
	permissionCache = cache.NewCache[bool](1 * time.Minute)                          // for permission keys
	rulesCache      = cache.NewCache[map[string]FrontendAccessRule](1 * time.Minute) // for rules
)

const accessRulesBucket = "access_rules"
const accessRulesKey = "rules"
const accessChangedKey = "newRule:"

type RuleMap map[string]*AccessRule
type SourceRuleMap map[string]RuleMap

type StringSet map[string]struct{}

type dbStorage struct {
	AllRules      SourceRuleMap       `json:"all_rules"`
	Groups        GroupMap            `json:"groups"`
	RevokedTokens map[string]struct{} `json:"revoked_tokens"` // set of revoked token hashes
	HashedTokens  map[string]uint     `json:"hashed_tokens"`  // maps token hash → user ID
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
	PathExists        bool            `json:"pathExists"`
}

// GroupMap maps group names to a set of usernames.
type GroupMap map[string]StringSet

// Storage manages access rules and group membership.
type Storage struct {
	mux           sync.RWMutex
	AllRules      SourceRuleMap       // AllRules[sourcePath][indexPath] - in-memory authoritative state
	Groups        GroupMap            // key: group name, value: set of usernames - in-memory authoritative state
	RevokedTokens map[string]struct{} // set of revoked token hashes - in-memory authoritative state
	HashedTokens  map[string]uint     // maps token hash → user ID - in-memory authoritative state
	DB            *storm.DB           // Optional: DB for persistence
	Users         *users.Storage      // Reference to users storage
}

// SaveToDB persists all rules to the DB if DB is set.
// IMPORTANT: Caller must hold s.mux lock (either read or write).
func (s *Storage) SaveToDB() error {
	if s.DB == nil {
		return nil
	}
	data, err := json.Marshal(&dbStorage{
		AllRules:      s.AllRules,
		Groups:        s.Groups,
		RevokedTokens: s.RevokedTokens,
		HashedTokens:  s.HashedTokens,
	})
	if err != nil {
		return err
	}
	return s.DB.Set(accessRulesBucket, accessRulesKey, data)
}

// Flush persists the current in-memory state to the backing store.
// Call during graceful shutdown to ensure DB matches memory.
func (s *Storage) Flush() error {
	s.mux.RLock()
	defer s.mux.RUnlock()
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
	s.RevokedTokens = storage.RevokedTokens
	if s.RevokedTokens == nil {
		s.RevokedTokens = make(map[string]struct{})
	}
	s.HashedTokens = storage.HashedTokens
	if s.HashedTokens == nil {
		s.HashedTokens = make(map[string]uint)
	}
	s.HashedTokens = storage.HashedTokens
	if s.HashedTokens == nil {
		s.HashedTokens = make(map[string]uint)
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
		AllRules:      make(SourceRuleMap),
		Groups:        make(GroupMap),
		RevokedTokens: make(map[string]struct{}),
		HashedTokens:  make(map[string]uint),
		DB:            db,
		Users:         usersStore,
	}
	return s
}

// ClearCache clears the access cache (useful for testing)
func ClearCache() {
	// Recreate the caches to clear them
	accessCache = cache.NewCache[string](1 * time.Minute)
	versionCache = cache.NewCache[int](1 * time.Minute)
	permissionCache = cache.NewCache[bool](1 * time.Minute)
	rulesCache = cache.NewCache[map[string]FrontendAccessRule](1 * time.Minute)
}

// clearAllCaches clears ALL caches. This should be called whenever rules are created, updated, or deleted.
func (s *Storage) clearAllCaches() {
	accessCache.ClearAll()
	versionCache.ClearAll()
	permissionCache.ClearAll()
	rulesCache.ClearAll()
}

// RemoveRuleByPath removes a rule by its exact path from the internal storage
func (s *Storage) RemoveRuleByPath(sourcePath, indexPath string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return
	}

	// Remove the rule by exact path match (don't normalize)
	if _, exists := rulesBySource[indexPath]; exists {
		delete(rulesBySource, indexPath)
		// If no rules left for this source, remove the source entry
		if len(rulesBySource) == 0 {
			delete(s.AllRules, sourcePath)
		}
		s.clearAllCaches()
		err := s.SaveToDB()
		if err != nil {
			logger.Errorf("error saving access rules to database: %v", err)
		}
	}
}

// getOrCreateRuleNL ensures a rule exists for the given source and index path.
// The caller must hold the lock.
func (s *Storage) getOrCreateRuleNL(sourcePath, indexPath string) *AccessRule {
	// Normalize the path to ensure consistent rule storage
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	if _, ok := s.AllRules[sourcePath]; !ok {
		s.AllRules[sourcePath] = make(RuleMap)
	}
	rule, ok := s.AllRules[sourcePath][normalizedPath]
	if !ok {
		rule = &AccessRule{
			Deny:  RuleSet{Users: make(StringSet), Groups: make(StringSet)},
			Allow: RuleSet{Users: make(StringSet), Groups: make(StringSet)},
		}
		s.AllRules[sourcePath][normalizedPath] = rule
	}
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
	s.clearAllCaches()
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
	s.clearAllCaches()
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
	s.clearAllCaches()
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
	s.clearAllCaches()
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
	s.clearAllCaches()
	return s.SaveToDB()
}

// Permitted checks if a username is permitted for a given sourcePath and indexPath, recursively checking parent directories.
func (s *Storage) Permitted(sourcePath, indexPath, username string) bool {
	// Ensure leading slash
	if !strings.HasPrefix(indexPath, "/") {
		indexPath = "/" + indexPath
	}
	indexPath = utils.AddTrailingSlashIfNotExists(indexPath)

	// Get current version for the sourcePath
	versionKey := "version:" + sourcePath
	version := 0
	if v, ok := versionCache.Get(versionKey); ok {
		version = v
	}

	// Check cache with versioned key
	permKey := fmt.Sprintf("perm:%s:%d:%s:%s", sourcePath, version, indexPath, username)
	if p, ok := permissionCache.Get(permKey); ok {
		return p
	}

	// Not in cache, compute, then cache it.
	result := s.computePermitted(sourcePath, indexPath, username)

	permissionCache.Set(permKey, result)
	return result
}

func (s *Storage) computePermitted(sourcePath, indexPath, username string) bool {
	var rulesFound []*AccessRule

	// Walk up the path hierarchy and collect all relevant rules
	currentPath := indexPath
	pathLevel := 0
	for {
		rule, found := s.getRuleAtExactPath(sourcePath, currentPath)
		if found {
			rulesFound = append(rulesFound, rule)
		}
		if currentPath == "/" || currentPath == "." || currentPath == "" {
			break
		}
		oldPath := currentPath
		currentPath = utils.GetParentDirectoryPath(currentPath)

		// Safety check to prevent infinite loops
		if currentPath == oldPath {
			break
		}
		pathLevel++
	}

	// Now evaluate the rules, starting from the most specific (indexPath) to the least specific (root)
	for _, rule := range rulesFound {
		permitted, hasSpecificRule := s.evaluateRuleForUser(rule, username)
		if hasSpecificRule {
			return permitted
		}
	}

	// No specific user or group rule found in the hierarchy.
	// Check for any DenyAll rule in the path.
	for _, rule := range rulesFound {
		if rule.DenyAll {
			return false
		}
	}

	// No specific rules found anywhere in the path hierarchy.
	// Fallback to the source's DenyByDefault setting.
	sourceInfo, ok := settings.Config.Server.SourceMap[sourcePath]
	if !ok {
		logger.Errorf("source %s not found in config during access check", sourcePath)
		return false
	}

	return !sourceInfo.Config.DenyByDefault
}

// getRuleAtExactPath is a helper to get a rule without the recursive logic.
func (s *Storage) getRuleAtExactPath(sourcePath, indexPath string) (*AccessRule, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return nil, false
	}
	// Normalize the path to ensure consistent rule lookup
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rule, ok := rulesBySource[normalizedPath]
	return rule, ok
}

// evaluateRuleForUser evaluates a single rule for a user and returns if a specific rule was found.
func (s *Storage) evaluateRuleForUser(rule *AccessRule, username string) (permitted bool, hasSpecificRule bool) {
	// Check user deny first
	if _, found := rule.Deny.Users[username]; found {
		return false, true
	}

	// Check group deny
	for group := range rule.Deny.Groups {
		if s.isUserInGroup(username, group) {
			return false, true
		}
	}

	// Check user allow
	if _, found := rule.Allow.Users[username]; found {
		return true, true
	}

	// Check group allow
	for group := range rule.Allow.Groups {
		if s.isUserInGroup(username, group) {
			return true, true
		}
	}

	// No specific rule for this user in this rule set.
	return false, false
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

	// Check if path exists on filesystem
	pathExists := utils.CheckPathExists(filepath.Join(sourcePath, indexPath))

	frontendRules := FrontendAccessRule{
		SourceDenyDefault: sourceDenyDefault,
		PathExists:        pathExists,
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
	return frontendRules, ok
}

// GetAllRules returns all access rules as a map.
func (s *Storage) GetAllRules(sourcePath string) (map[string]FrontendAccessRule, error) {
	// Check if rules have changed by looking at the access cache
	_, hasChanged := accessCache.Get(accessChangedKey + sourcePath)
	if !hasChanged {
		// If no change marker, check if we have cached rules
		value, ok := rulesCache.Get(accessChangedKey + sourcePath)
		if ok {
			return value, nil
		}
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
		// Use the internal path as the frontend path (with trailing slash)
		// This ensures consistency between internal storage and frontend display
		frontendPath := indexPath

		// Check if path exists on filesystem
		pathExists := utils.CheckPathExists(filepath.Join(sourcePath, indexPath))

		// Convert AccessRule to FrontendAccessRule
		frontendRules[frontendPath] = FrontendAccessRule{
			DenyAll:           rule.DenyAll,
			SourceDenyDefault: sourceDenyDefault,
			PathExists:        pathExists,
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
	rulesCache.Set(accessChangedKey+sourcePath, frontendRules)
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
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rule, ok := s.AllRules[sourcePath][normalizedPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Allow.Users[username]
	if exists {
		delete(rule.Allow.Users, username)
	}
	removed := exists
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], normalizedPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		s.clearAllCaches()
		return exists, s.SaveToDB()
	}
	return false, nil
}

// RemoveAllowGroup removes a group from the allow list for a given source and index path.
func (s *Storage) RemoveAllowGroup(sourcePath, indexPath, groupname string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rule, ok := s.AllRules[sourcePath][normalizedPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Allow.Groups[groupname]
	if exists {
		delete(rule.Allow.Groups, groupname)
	}
	removed := exists
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], normalizedPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		s.clearAllCaches()
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveDenyUser removes a user from the deny list for a given source and index path.
func (s *Storage) RemoveDenyUser(sourcePath, indexPath, username string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rule, ok := s.AllRules[sourcePath][normalizedPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Deny.Users[username]
	if exists {
		delete(rule.Deny.Users, username)
	}
	removed := exists
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], normalizedPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		s.clearAllCaches()
		return exists, s.SaveToDB()
	}
	return false, nil
}

// RemoveDenyGroup removes a group from the deny list for a given source and index path.
func (s *Storage) RemoveDenyGroup(sourcePath, indexPath, groupname string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rule, ok := s.AllRules[sourcePath][normalizedPath]
	if !ok {
		return false, nil
	}
	_, exists := rule.Deny.Groups[groupname]
	if exists {
		delete(rule.Deny.Groups, groupname)
	}
	removed := exists
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], normalizedPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		s.clearAllCaches()
		return exists, s.SaveToDB()
	}
	return exists, nil
}

// RemoveDenyAll removes the deny all rule for a given source and index path.
func (s *Storage) RemoveDenyAll(sourcePath, indexPath string) (bool, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rule, ok := s.AllRules[sourcePath][normalizedPath]
	if !ok {
		return false, nil
	}
	removed := false
	if rule.DenyAll {
		rule.DenyAll = false
		removed = true
	}
	// If rule is now empty, remove it
	if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 && len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 {
		delete(s.AllRules[sourcePath], normalizedPath)
		if len(s.AllRules[sourcePath]) == 0 {
			delete(s.AllRules, sourcePath)
		}
	}
	if removed {
		s.clearAllCaches()
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
	if changed {
		s.clearAllCaches()
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
	if changed {
		s.clearAllCaches()
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

		// Use the internal path as the frontend path (with trailing slash)
		// This ensures consistency between internal storage and frontend display
		frontendPath := indexPath

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
			allUserRules[user][frontendPath] = frontendRule
		}
		for user := range rule.Deny.Users {
			if _, ok := allUserRules[user]; !ok {
				allUserRules[user] = make(map[string]FrontendAccessRule)
			}
			allUserRules[user][frontendPath] = frontendRule
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

		// Use the internal path as the frontend path (with trailing slash)
		// This ensures consistency between internal storage and frontend display
		frontendPath := indexPath

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
			allGroupRules[group][frontendPath] = frontendRule
		}
		for group := range rule.Deny.Groups {
			if _, ok := allGroupRules[group]; !ok {
				allGroupRules[group] = make(map[string]FrontendAccessRule)
			}
			allGroupRules[group][frontendPath] = frontendRule
		}
	}
	return allGroupRules
}

// HasAnyVisibleItems checks if a user has access to any items in a given parent path.
// This is used to determine if a user should see a folder's contents even when
// they don't have direct access to the parent folder.
func (s *Storage) HasAnyVisibleItems(sourcePath, parentPath string, itemNames []string, username string) bool {
	parentPath = utils.AddTrailingSlashIfNotExists(parentPath)
	// Check if user has access to any of the items
	for _, itemName := range itemNames {
		indexPath := parentPath + itemName
		if s.Permitted(sourcePath, indexPath, username) {
			return true
		}
	}

	return false
}

// RemoveUserCascade removes a user from either the allow or deny list for a given path and all its subpaths.
// This is used for cascade delete operations when deleting user access from a directory tree.
// The allow parameter determines which list to remove from: true for allow list, false for deny list.
func (s *Storage) RemoveUserCascade(sourcePath, indexPath, username string, allow bool) (int, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return 0, nil
	}

	changed := false
	removedCount := 0

	// Iterate through all rules for this source
	for rulePath, rule := range rulesBySource {
		// Check if this rule path matches or is a subpath of the target path
		if rulePath == normalizedPath || strings.HasPrefix(rulePath, normalizedPath) {
			if allow {
				// Remove user from allow list only
				if _, exists := rule.Allow.Users[username]; exists {
					delete(rule.Allow.Users, username)
					changed = true
					removedCount++
				}
			} else {
				// Remove user from deny list only
				if _, exists := rule.Deny.Users[username]; exists {
					delete(rule.Deny.Users, username)
					changed = true
					removedCount++
				}
			}

			// If rule is now empty, mark it for deletion
			if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 &&
				len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 && !rule.DenyAll {
				delete(s.AllRules[sourcePath], rulePath)
			}
		}
	}

	// If no rules left for this source, remove the source entry
	if len(s.AllRules[sourcePath]) == 0 {
		delete(s.AllRules, sourcePath)
	}

	if changed {
		s.clearAllCaches()
		return removedCount, s.SaveToDB()
	}

	return 0, nil
}

// RemoveGroupCascade removes a group from either the allow or deny list for a given path and all its subpaths.
// This is used for cascade delete operations when deleting group access from a directory tree.
// The allow parameter determines which list to remove from: true for allow list, false for deny list.
func (s *Storage) RemoveGroupCascade(sourcePath, indexPath, groupname string, allow bool) (int, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	normalizedPath := utils.AddTrailingSlashIfNotExists(indexPath)
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return 0, nil
	}

	changed := false
	removedCount := 0

	// Iterate through all rules for this source
	for rulePath, rule := range rulesBySource {
		// Check if this rule path matches or is a subpath of the target path
		if rulePath == normalizedPath || strings.HasPrefix(rulePath, normalizedPath) {
			if allow {
				// Remove group from allow list only
				if _, exists := rule.Allow.Groups[groupname]; exists {
					delete(rule.Allow.Groups, groupname)
					changed = true
					removedCount++
				}
			} else {
				// Remove group from deny list only
				if _, exists := rule.Deny.Groups[groupname]; exists {
					delete(rule.Deny.Groups, groupname)
					changed = true
					removedCount++
				}
			}

			// If rule is now empty, mark it for deletion
			if len(rule.Allow.Users) == 0 && len(rule.Allow.Groups) == 0 &&
				len(rule.Deny.Users) == 0 && len(rule.Deny.Groups) == 0 && !rule.DenyAll {
				delete(s.AllRules[sourcePath], rulePath)
			}
		}
	}

	// If no rules left for this source, remove the source entry
	if len(s.AllRules[sourcePath]) == 0 {
		delete(s.AllRules, sourcePath)
	}

	if changed {
		s.clearAllCaches()
		return removedCount, s.SaveToDB()
	}

	return 0, nil
}

// UpdateRules updates all access rules that match oldPath to point to newPath.
// Handles both exact matches and subdirectories. Similar to share.Storage.UpdateShares.
func (s *Storage) UpdateRules(sourcePath, oldPath, newPath string) (int, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return 0, nil // No rules for this source, not an error
	}

	// Normalize paths (always add trailing slash for access rules)
	oldPath = utils.AddTrailingSlashIfNotExists(oldPath)
	newPath = utils.AddTrailingSlashIfNotExists(newPath)

	updated := 0
	rulesToUpdate := make(map[string]string) // old path -> new path

	// Find all rules that need to be updated
	for rulePath := range rulesBySource {
		if rulePath == oldPath {
			// Exact match
			rulesToUpdate[rulePath] = newPath
		} else if strings.HasPrefix(rulePath, oldPath) {
			// Subdirectory - replace prefix
			newRulePath := newPath + strings.TrimPrefix(rulePath, oldPath)
			rulesToUpdate[rulePath] = newRulePath
		}
	}

	// Update all matched rules
	for oldRulePath, newRulePath := range rulesToUpdate {
		rule := rulesBySource[oldRulePath]
		delete(rulesBySource, oldRulePath)
		rulesBySource[newRulePath] = rule
		logger.Info("access rule updated", "source", sourcePath, "fromPath", oldRulePath, "toPath", newRulePath)
		updated++
	}

	if updated > 0 {
		s.clearAllCaches()
		if err := s.SaveToDB(); err != nil {
			return updated, err
		}
	}

	return updated, nil
}

// UpdateRulePath updates the path for a specific access rule (used by PATCH API endpoint).
func (s *Storage) UpdateRulePath(sourcePath, oldPath, newPath string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// Normalize paths
	oldPath = utils.AddTrailingSlashIfNotExists(oldPath)
	newPath = utils.AddTrailingSlashIfNotExists(newPath)

	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok {
		return fmt.Errorf("no rules found for source: %s", sourcePath)
	}

	rule, ok := rulesBySource[oldPath]
	if !ok {
		return fmt.Errorf("no rule found for path: %s", oldPath)
	}

	// Remove the old rule and add it with the new path
	delete(rulesBySource, oldPath)
	rulesBySource[newPath] = rule
	s.clearAllCaches()
	logger.Debugf("access rule path updated: source=%s, fromPath=%s, toPath=%s", sourcePath, oldPath, newPath)
	return s.SaveToDB()
}

// RevokeToken adds a token hash to the revoked list and persists to DB.
func (s *Storage) RevokeToken(tokenHash string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	tokenHash = utils.HashSHA256(tokenHash)
	s.RevokedTokens[tokenHash] = struct{}{}
	// Also remove from HashedTokens to prevent future lookups
	delete(s.HashedTokens, tokenHash)
	return s.SaveToDB()
}

// IsTokenRevoked checks if a token hash is in the revoked list (memory-only read).
func (s *Storage) IsTokenRevoked(tokenHash string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	tokenHash = utils.HashSHA256(tokenHash)
	_, exists := s.RevokedTokens[tokenHash]
	return exists
}

// AddHashedToken adds a token hash to user ID mapping.
func (s *Storage) AddApiToken(tokenString string, userID uint) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	tokenHash := utils.HashSHA256(tokenString)
	s.HashedTokens[tokenHash] = userID
	return s.SaveToDB()
}

// GetUserIDFromToken retrieves the user ID for a given token string (memory-only read).
func (s *Storage) GetUserIDFromToken(tokenString string) (uint, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	tokenHash := utils.HashSHA256(tokenString)
	userID, exists := s.HashedTokens[tokenHash]
	return userID, exists
}

// GetRevokedTokens returns a copy of all revoked token hashes.
func (s *Storage) GetRevokedTokens() map[string]struct{} {
	s.mux.RLock()
	defer s.mux.RUnlock()
	// Return a copy to prevent external modification
	result := make(map[string]struct{}, len(s.RevokedTokens))
	for k := range s.RevokedTokens {
		result[k] = struct{}{}
	}
	return result
}

// RemoveApiToken removes a token hash mapping (used when deleting API keys).
func (s *Storage) RemoveApiToken(tokenString string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	tokenHash := utils.HashSHA256(tokenString)
	delete(s.HashedTokens, tokenHash)
	return s.SaveToDB()
}
