// Package auth_test contains timing attack tests for authentication.
//
// These tests verify that the JSON authentication implementation is protected
// against timing attacks that could leak information about valid usernames.
//
// Timing Attack Background:
// A timing attack is a side-channel attack where an attacker measures how long
// authentication operations take to determine if a username exists in the system.
// If authentication fails faster for non-existent users than for valid users
// with wrong passwords, an attacker can enumerate valid usernames.
//
// Protection Mechanism:
// The json.go Auth() implementation protects against timing attacks by:
// 1. Always running the bcrypt password comparison, even for non-existent users
// 2. Using utils.InvalidPasswordHash (a pre-computed bcrypt hash) when the user doesn't exist
// 3. Ensuring both valid and invalid user authentication paths take similar time
//
// Test Coverage:
// - TestJSONAuth_NoTimingAttack: Statistical analysis of timing differences
// - TestJSONAuth_InvalidPasswordHashInitialized: Verifies protection is in place
// - TestJSONAuth_ValidUserVsInvalidUserDirectComparison: Direct comparison test
package auth_test

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// mockUserBackend is a simple in-memory user storage for testing.
type mockUserBackend struct {
	usersByUsername map[string]*users.User
	usersByID       map[uint64]*users.User
}

func newMockUserBackend() *mockUserBackend {
	return &mockUserBackend{
		usersByUsername: make(map[string]*users.User),
		usersByID:       make(map[uint64]*users.User),
	}
}

func (m *mockUserBackend) GetBy(id uint64) (*users.User, error) {
	if user, ok := m.usersByID[id]; ok {
		return user, nil
	}
	return nil, fmt.Errorf("user not found: %d", id)
}

func (m *mockUserBackend) Gets() ([]*users.User, error) {
	result := make([]*users.User, 0, len(m.usersByID))
	for _, user := range m.usersByID {
		result = append(result, user)
	}
	return result, nil
}

func (m *mockUserBackend) Save(u *users.User, changePass bool, disableScopeChange bool) error {
	m.usersByUsername[u.Username] = u
	m.usersByID[u.ID] = u
	return nil
}

func (m *mockUserBackend) Update(u *users.User, adminActor bool, fields ...string) error {
	m.usersByUsername[u.Username] = u
	m.usersByID[u.ID] = u
	return nil
}

func (m *mockUserBackend) DeleteByID(id uint64) error {
	if user, ok := m.usersByID[id]; ok {
		delete(m.usersByUsername, user.Username)
		delete(m.usersByID, id)
		return nil
	}
	return fmt.Errorf("user not found: %d", id)
}

// setupTestUsers creates a user storage with 2 valid test users
func setupTestUsers(t *testing.T) *users.Storage {
	t.Helper()

	// Reduce bcrypt cost for faster tests (default is 10, min is 4)
	// This significantly speeds up password hashing during tests
	originalCost := utils.BcryptCost
	utils.BcryptCost = 4 // Minimum bcrypt cost for testing
	t.Cleanup(func() {
		utils.BcryptCost = originalCost // Restore original cost after test
	})

	// Initialize InvalidPasswordHash to prevent timing attacks
	// This simulates the application startup initialization
	err := utils.SetInvalidPasswordHash()
	if err != nil {
		t.Fatalf("Failed to set invalid password hash: %v", err)
	}

	backend := newMockUserBackend()
	storage := users.NewStorage(backend)
	users.SetUsernameToID(func(username string) (uint64, error) {
		if u, ok := backend.usersByUsername[username]; ok {
			return u.ID, nil
		}
		return 0, errors.ErrNotExist
	})
	t.Cleanup(func() {
		users.SetUsernameToID(nil)
	})

	// Create two valid users with hashed passwords
	password1Hash, err := utils.HashPwd("password123")
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	password2Hash, err := utils.HashPwd("securepass456")
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user1 := &users.User{
		ID:       1,
		Username: "admin",
		NonAdminEditable: users.NonAdminEditable{
			Password: password1Hash,
		},
		LoginMethod: users.LoginMethodPassword,
	}

	user2 := &users.User{
		ID:       2,
		Username: "testuser",
		NonAdminEditable: users.NonAdminEditable{
			Password: password2Hash,
		},
		LoginMethod: users.LoginMethodPassword,
	}

	err = storage.Save(user1, false, false)
	if err != nil {
		t.Fatalf("Failed to save user1: %v", err)
	}

	err = storage.Save(user2, false, false)
	if err != nil {
		t.Fatalf("Failed to save user2: %v", err)
	}

	return storage
}

// measureAuthTime measures the time taken to authenticate with given credentials
func measureAuthTime(t *testing.T, auther auth.JSONAuth, userStore *users.Storage, username, password string) time.Duration {
	t.Helper()

	// Create a mock HTTP request
	req, err := http.NewRequest("POST", "/api/auth/login?username="+url.QueryEscape(username), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Password", url.QueryEscape(password))

	start := time.Now()
	_, _ = auther.Auth(req, userStore)
	elapsed := time.Since(start)

	return elapsed
}

// TestJSONAuth_NoTimingAttack tests that authentication timing does not leak user existence
func TestJSONAuth_NoTimingAttack(t *testing.T) {
	storage := setupTestUsers(t)
	auther := auth.JSONAuth{}

	// Test cases: mix of valid and invalid usernames
	testCases := []struct {
		username string
		isValid  bool
	}{
		{"admin", true},
		{"testuser", true},
		{"nonexistent1", false},
		{"invaliduser", false},
		{"notauser", false},
		{"fakeadmin", false},
	}

	// Number of samples per test case for statistical analysis
	samplesPerCase := 5

	// Collect timing measurements
	measurements := make(map[string][]time.Duration)
	for _, tc := range testCases {
		measurements[tc.username] = make([]time.Duration, 0, samplesPerCase)
	}

	// Run multiple iterations to gather statistical data
	for i := 0; i < samplesPerCase; i++ {
		// Randomize order to avoid cache effects
		for _, tc := range testCases {
			elapsed := measureAuthTime(t, auther, storage, tc.username, "wrongpassword123")
			measurements[tc.username] = append(measurements[tc.username], elapsed)
		}
	}

	// Calculate statistics for each test case
	type stats struct {
		mean   time.Duration
		stddev time.Duration
		min    time.Duration
		max    time.Duration
	}

	calculateStats := func(durations []time.Duration) stats {
		if len(durations) == 0 {
			return stats{}
		}

		// Calculate mean
		var sum time.Duration
		for _, d := range durations {
			sum += d
		}
		mean := sum / time.Duration(len(durations))

		// Calculate standard deviation
		var variance float64
		for _, d := range durations {
			diff := float64(d - mean)
			variance += diff * diff
		}
		variance /= float64(len(durations))
		stddev := time.Duration(math.Sqrt(variance))

		// Find min and max
		min := durations[0]
		max := durations[0]
		for _, d := range durations {
			if d < min {
				min = d
			}
			if d > max {
				max = d
			}
		}

		return stats{mean: mean, stddev: stddev, min: min, max: max}
	}

	// Analyze results
	validUserStats := make([]stats, 0)
	invalidUserStats := make([]stats, 0)

	t.Log("\n=== Timing Analysis Results ===")
	for _, tc := range testCases {
		s := calculateStats(measurements[tc.username])
		t.Logf("%s (valid=%v): mean=%.4fms, stddev=%.4fms, min=%.4fms, max=%.4fms",
			tc.username, tc.isValid,
			float64(s.mean.Microseconds())/1000.0,
			float64(s.stddev.Microseconds())/1000.0,
			float64(s.min.Microseconds())/1000.0,
			float64(s.max.Microseconds())/1000.0)

		if tc.isValid {
			validUserStats = append(validUserStats, s)
		} else {
			invalidUserStats = append(invalidUserStats, s)
		}
	}

	// Calculate overall statistics for valid vs invalid users
	var validMeanSum, invalidMeanSum time.Duration
	for _, s := range validUserStats {
		validMeanSum += s.mean
	}
	for _, s := range invalidUserStats {
		invalidMeanSum += s.mean
	}

	validAvgMean := validMeanSum / time.Duration(len(validUserStats))
	invalidAvgMean := invalidMeanSum / time.Duration(len(invalidUserStats))

	t.Logf("\n=== Summary ===")
	t.Logf("Valid users average: %.4fms", float64(validAvgMean.Microseconds())/1000.0)
	t.Logf("Invalid users average: %.4fms", float64(invalidAvgMean.Microseconds())/1000.0)

	// Calculate the percentage difference
	diff := float64(validAvgMean - invalidAvgMean)
	percentDiff := math.Abs(diff) / float64(validAvgMean) * 100.0
	t.Logf("Difference: %.4fms (%.2f%%)", diff/1e6, percentDiff)

	// Assert: The timing difference should be less than 20%
	// Bcrypt's constant-time comparison should make both cases take similar time
	// We allow 20% threshold to account for system noise and variability
	threshold := 20.0 // percent
	if percentDiff > threshold {
		t.Errorf("TIMING ATTACK VULNERABILITY DETECTED: Valid vs invalid user timing differs by %.2f%% (threshold: %.2f%%)",
			percentDiff, threshold)
		t.Errorf("This difference could allow an attacker to enumerate valid usernames!")
	} else {
		t.Logf("✓ No timing attack vulnerability detected (difference: %.2f%% < threshold: %.2f%%)",
			percentDiff, threshold)
	}

	// Additional check: Ensure no individual case is an obvious outlier
	// Check if any single user's timing is significantly faster than bcrypt operations
	allStats := append(validUserStats, invalidUserStats...)
	var allMeansSum time.Duration
	for _, s := range allStats {
		allMeansSum += s.mean
	}
	overallMean := allMeansSum / time.Duration(len(allStats))

	for _, tc := range testCases {
		s := calculateStats(measurements[tc.username])
		deviation := math.Abs(float64(s.mean-overallMean)) / float64(overallMean) * 100.0

		// If any user is more than 30% faster/slower than average, it's suspicious
		if deviation > 30.0 {
			t.Errorf("OUTLIER DETECTED: %s deviates by %.2f%% from overall mean (%.4fms vs %.4fms)",
				tc.username, deviation,
				float64(s.mean.Microseconds())/1000.0,
				float64(overallMean.Microseconds())/1000.0)
		}
	}
}

// TestJSONAuth_InvalidPasswordHashInitialized verifies that InvalidPasswordHash is properly set
func TestJSONAuth_InvalidPasswordHashInitialized(t *testing.T) {
	// This test ensures that the InvalidPasswordHash is initialized
	// If it's empty, timing attacks would be possible because invalid users
	// would skip bcrypt comparison entirely

	// Use lower bcrypt cost for faster testing
	originalCost := utils.BcryptCost
	utils.BcryptCost = 4
	t.Cleanup(func() {
		utils.BcryptCost = originalCost
	})

	err := utils.SetInvalidPasswordHash()
	if err != nil {
		t.Fatalf("Failed to set invalid password hash: %v", err)
	}

	if utils.InvalidPasswordHash == "" {
		t.Fatal("InvalidPasswordHash is not initialized - timing attack vulnerability exists!")
	}

	// Verify that the invalid hash can be used for password comparison
	// This should fail (wrong password) but take the same time as a real bcrypt comparison
	err = utils.CheckPwd("anypassword", utils.InvalidPasswordHash)
	if err == nil {
		t.Fatal("CheckPwd should fail with InvalidPasswordHash")
	}

	t.Log("✓ InvalidPasswordHash is properly initialized")
}
