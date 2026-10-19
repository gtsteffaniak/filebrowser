package access

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	accessCache     = cache.NewCache[string](1 * time.Minute)                        // for accessChangedKey
	versionCache    = cache.NewCache[int](1 * time.Minute)                           // for version keys
	permissionCache = cache.NewCache[bool](1 * time.Minute)                          // for permission keys
	rulesCache      = cache.NewCache[map[string]FrontendAccessRule](1 * time.Minute) // for rules
)

const accessChangedKey = "newRule:"

type RuleMap map[string]*AccessRule
type SourceRuleMap map[string]RuleMap

type StringSet map[string]struct{}

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

// HashedTokenInfo describes a registered bearer token owner and type.
type HashedTokenInfo struct {
	UserID    uint64
	IsSession bool
	// ExpiresAt is the token's exp claim as unix t