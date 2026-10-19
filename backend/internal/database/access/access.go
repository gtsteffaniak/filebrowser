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
	// ExpiresAt is the token's exp claim as unix time; 0 means no expiry (e.g.
	// non-JWT strings or rows persisted before expiry tracking).
	ExpiresAt int64
}

// BearerTokenGrace is how long a bearer token may still resolve after session
// rotation or after its JWT exp (e.g. share-ACL identity). In-flight requests
// still carrying the previous cookie must not 401 during rotation.
const BearerTokenGrace = 2 * time.Minute

// TokenExpiryUnix extracts the exp claim of a bearer JWT without verifying the
// signature; the value is only used for expiry bookkeeping and tokens are
// verified on the auth path. Returns 0 for non-JWT strings or tokens without
// an expiry claim.
func TokenExpiryUnix(tokenString string) int64 {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tokenString, claims); err != nil {
		return 0
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return 0
	}
	return int64(exp)
}

// TokenExpiredPastGrace reports whether a token expiry is older than the
// resolution grace window. expiresAt == 0 (unknown/none) never expires.
func TokenExpiredPastGrace(expiresAt int64, now time.Time) bool {
	return expiresAt != 0 && now.After(time.Unix(expiresAt, 0).Add(BearerTokenGrace))
}

// Storage manages access rules and group membership.
type Storage struct {
	mux                   sync.RWMutex
	AllRules              SourceRuleMap // AllRules[sourcePath][indexPath] - in-memory authoritative state
	sourceCaseInsensitive map[string]bool // per-source: lowercase rule keys and lookups
	Groups                GroupMap        // key: group name, value: set of usernames - in-memory authoritative state
	RevokedTokens map[string]int64           // token hash → revocation unix time (0 = immediate) - in-memory authoritative state
	HashedTokens  map[string]HashedTokenInfo // maps token hash → owner/type - in-memory authoritative state
	Users         *users.Storage             // Reference to users storage
	sqlStore      SQLPersister               // SQL store for persistence
}

// SQLPersister interface for SQL persistence operations
type SQLPersister interface {
	SaveAccessRule(source, path string, rule *AccessRule) error
	DeleteAccessRule(source, path string) error
	SaveGroup(name string, members StringSet) error
	DeleteGroup(name string) error
	SaveRevokedToken(tokenHash string, revokedAt int64) error
	PersistImmediateTokenRevocation(tokenHash string) error
	PersistTokenRetirement(tokenHash string, revokedAt int64, pruned []string) error
	DeleteRevokedToken(tokenHash string) error
	SaveHashedToken(tokenHash string, userID uint64, isSession bool, expiresAt int64) error
	DeleteHashedToken(tokenHash string) error
	DeleteHashedTokensByUserID(userID uint64) error
}

// SetSQLStore sets the SQL store for persistence operations
func (s *Storage) SetSQLStore(sqlStore SQLPersister) {
	s.sqlStore = sqlStore
}

// Flush is a no-op; access rule and group updates already write through sqlStore when configured.
func (s *Storage) Flush() error {
	return nil
}

// persistGroupSQLNL upser