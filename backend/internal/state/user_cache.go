package state

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/go-cache/cache"
)

const userCacheTTL = 30 * time.Minute

var userRecordCache = cache.NewCache[*users.User](userCacheTTL, userCacheTTL+5*time.Minute)

func userCacheKeyID(id uint64) string {
	return fmt.Sprintf("id:%d", id)
}

func userCacheKeyName(username string) string {
	return fmt.Sprintf("name:%s", username)
}

func putUserInCache(user *users.User) {
	if user == nil {
		return
	}
	cached := cloneUserPtr(user)
	userRecordCache.Set(userCacheKeyName(user.Username), cached)
	if user.ID != 0 {
		userRecordCache.Set(userCacheKeyID(user.ID), cached)
	}
}

func deleteUserFromCache(user *users.User) {
	if user == nil {
		return
	}
	userRecordCache.Delete(userCacheKeyName(user.Username))
	if user.ID != 0 {
		userRecordCache.Delete(userCacheKeyID(user.ID))
	}
}

func clearUserRecordCache() {
	userRecordCache.ClearAll()
}

func cloneUserPtr(user *users.User) *users.User {
	if user == nil {
		return nil
	}
	copy := *user
	copyUserSlices(&copy, user)
	return &copy
}

func loadUserByUsernameFromDB(username string) (*users.User, error) {
	user, err := sqlDb.GetUserByUsername(username)
	if err != nil && err.Error() == "user not found" {
		return nil, errors.ErrNotExist
	}
	return user, err
}

func loadUserByIDFromDB(id uint64) (*users.User, error) {
	user, err := sqlDb.GetUserByID(id)
	if err != nil && err.Error() == "user not found" {
		return nil, errors.ErrNotExist
	}
	return user, err
}

func getCachedUserByUsername(username string) (*users.User, bool) {
	if cached, ok := userRecordCache.Get(userCacheKeyName(username)); ok && cached != nil {
		return cloneUserPtr(cached), true
	}
	return nil, false
}

func getCachedUserByID(id uint64) (*users.User, bool) {
	if cached, ok := userRecordCache.Get(userCacheKeyID(id)); ok && cached != nil {
		return cloneUserPtr(cached), true
	}
	return nil, false
}

// loadExistingUserLocked loads a mutable user copy for updates (callers must hold usersMux).
func loadExistingUserLocked(user *users.User) (*users.User, error) {
	if user == nil {
		return nil, fmt.Errorf("user not found in cache")
	}
	if user.ID != 0 {
		if cached, ok := getCachedUserByID(user.ID); ok {
			return cached, nil
		}
		fromDB, err := loadUserByIDFromDB(user.ID)
		if err != nil {
			return nil, err
		}
		return cloneUserPtr(fromDB), nil
	}
	if user.Username != "" {
		if cached, ok := getCachedUserByUsername(user.Username); ok {
			return cached, nil
		}
		fromDB, err := loadUserByUsernameFromDB(user.Username)
		if err != nil {
			return nil, err
		}
		return cloneUserPtr(fromDB), nil
	}
	return nil, fmt.Errorf("user not found in cache")
}
