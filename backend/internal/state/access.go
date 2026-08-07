package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// AccessPermitted reports whether username may access indexPath on sourcePath.
// When accessDb is not initialized, access is allowed.
func AccessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool {
	if accessDb == nil {
		return true
	}
	return accessDb.Permitted(sourcePath, indexPath, username)
}

func AllowUser(sourcePath string, indexPath utils.IndexPath, username string) error {
	return accessDb.AllowUser(sourcePath, indexPath, username)
}

func AllowGroup(sourcePath string, indexPath utils.IndexPath, groupname string) error {
	return accessDb.AllowGroup(sourcePath, indexPath, groupname)
}

func DenyUser(sourcePath string, indexPath utils.IndexPath, username string) error {
	return accessDb.DenyUser(sourcePath, indexPath, username)
}

func DenyGroup(sourcePath string, indexPath utils.IndexPath, groupname string) error {
	return accessDb.DenyGroup(sourcePath, indexPath, groupname)
}

func DenyAll(sourcePath string, indexPath utils.IndexPath) error {
	return accessDb.DenyAll(sourcePath, indexPath)
}

func GetFrontendRules(sourcePath string, indexPath utils.IndexPath) (access.FrontendAccessRule, bool) {
	return accessDb.GetFrontendRules(sourcePath, indexPath)
}

func GetAllRules(sourcePath string) (map[string]access.FrontendAccessRule, error) {
	return accessDb.GetAllRules(sourcePath)
}

func GetRulesForUser(sourcePath, username string) map[string]access.FrontendAccessRule {
	return accessDb.GetRulesForUser(sourcePath, username)
}

func GetRulesForGroup(sourcePath, groupname string) map[string]access.FrontendAccessRule {
	return accessDb.GetRulesForGroup(sourcePath, groupname)
}

func RemoveUserCascade(sourcePath string, indexPath utils.IndexPath, username string, allow bool) (int, error) {
	return accessDb.RemoveUserCascade(sourcePath, indexPath, username, allow)
}

func RemoveGroupCascade(sourcePath string, indexPath utils.IndexPath, groupname string, allow bool) (int, error) {
	return accessDb.RemoveGroupCascade(sourcePath, indexPath, groupname, allow)
}

func RemoveAllowUser(sourcePath string, indexPath utils.IndexPath, username string) (bool, error) {
	return accessDb.RemoveAllowUser(sourcePath, indexPath, username)
}

func RemoveAllowGroup(sourcePath string, indexPath utils.IndexPath, groupname string) (bool, error) {
	return accessDb.RemoveAllowGroup(sourcePath, indexPath, groupname)
}

func RemoveDenyUser(sourcePath string, indexPath utils.IndexPath, username string) (bool, error) {
	return accessDb.RemoveDenyUser(sourcePath, indexPath, username)
}

func RemoveDenyGroup(sourcePath string, indexPath utils.IndexPath, groupname string) (bool, error) {
	return accessDb.RemoveDenyGroup(sourcePath, indexPath, groupname)
}

func RemoveDenyAll(sourcePath string, indexPath utils.IndexPath) (bool, error) {
	return accessDb.RemoveDenyAll(sourcePath, indexPath)
}

func GetUserGroups(username string) []string {
	return accessDb.GetUserGroups(username)
}

func GetAllGroups() []string {
	return accessDb.GetAllGroups()
}

func AddUserToGroup(group, username string) error {
	return accessDb.AddUserToGroup(group, username)
}

func RemoveUserFromGroup(group, username string) error {
	return accessDb.RemoveUserFromGroup(group, username)
}

func SyncUserGroups(username string, newGroups []string) error {
	return accessDb.SyncUserGroups(username, newGroups)
}

func UpdateRulePath(sourcePath string, oldPath, newPath utils.IndexPath) error {
	return accessDb.UpdateRulePath(sourcePath, oldPath, newPath)
}

func RemoveRuleByPathKey(sourcePath, pathKey string) {
	accessDb.RemoveRuleByPathKey(sourcePath, pathKey)
}

func AddApiToken(tokenString string, userID uint64) error {
	return accessDb.AddApiToken(tokenString, userID)
}

func RemoveApiToken(tokenString string) error {
	return accessDb.RemoveApiToken(tokenString)
}

func IsTokenRevoked(token string) bool {
	if accessDb == nil {
		return false
	}
	return accessDb.IsTokenRevoked(token)
}

func RevokeToken(token string) error {
	return accessDb.RevokeToken(token)
}

// CheckChildItemAccess filters directory listings using in-memory access rules.
func CheckChildItemAccess(response *iteminfo.FileInfo, idx *indexing.Index, username string) error {
	if accessDb == nil {
		return nil
	}
	return accessDb.CheckChildItemAccess(response, idx, username)
}

// UpdateAccessRulesOnMove rewrites access rule paths when a resource moves within one source.
func UpdateAccessRulesOnMove(sourcePath string, oldPath, newPath utils.IndexPath) (int, error) {
	if accessDb == nil {
		return 0, nil
	}
	return accessDb.UpdateRules(sourcePath, oldPath, newPath)
}
