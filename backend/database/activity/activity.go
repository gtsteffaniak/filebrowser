package activity

import "encoding/json"

// Details holds metadata stored as JSON in the database.
type Details struct {
	TargetUsername   string        `json:"targetUsername,omitempty"`
	Source           string        `json:"source,omitempty"`
	Path             string        `json:"path,omitempty"`
	TargetPath       string        `json:"targetPath,omitempty"`
	ShareHash        string        `json:"shareHash,omitempty"`
	ShareOwnerUserID uint64        `json:"shareOwnerUserId,omitempty"`
	Scopes           []ScopeDetail `json:"scopes,omitempty"`
	TokenName         string        `json:"tokenName,omitempty"` // actor token; API exposes on FrontendEntry only
	AffectedTokenName string        `json:"affectedTokenName,omitempty"` // token created/deleted (actor token is TokenName)
	AuthMethod        string        `json:"authMethod,omitempty"` // actor auth; API exposes on FrontendEntry only
	LoginMethod      string        `json:"loginMethod,omitempty"`
	PasskeyName      string        `json:"passkeyName,omitempty"`
	UpdatedFields    []string      `json:"updatedFields,omitempty"`
	Changes          []FieldChange `json:"changes,omitempty"`
	Cached           bool          `json:"cached,omitempty"`
	FileCount        int           `json:"fileCount,omitempty"`
	Paths            []string      `json:"paths,omitempty"`
	Truncated        bool          `json:"truncated,omitempty"`
	Bytes            int64         `json:"bytes,omitempty"`
	DurationMs       int64         `json:"durationMs,omitempty"`
	Error            string        `json:"error,omitempty"`
}

// ScopeDetail is a user source + path scope for admin/user mutation events.
type ScopeDetail struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

// FieldChange records one attribute that changed on user/share update events.
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
}

// FrontendDetails is the admin-only detail payload exposed to the API.
type FrontendDetails struct {
	TargetUsername string        `json:"targetUsername,omitempty"`
	Source         string        `json:"source,omitempty"`
	Path           string        `json:"path,omitempty"`
	TargetPath     string        `json:"targetPath,omitempty"`
	Scopes         []ScopeDetail `json:"scopes,omitempty"`
	AffectedTokenName string        `json:"affectedTokenName,omitempty"`
	LoginMethod    string        `json:"loginMethod,omitempty"`
	PasskeyName    string        `json:"passkeyName,omitempty"`
	UpdatedFields  []string      `json:"updatedFields,omitempty"`
	Changes        []FieldChange `json:"changes,omitempty"`
	Cached         bool          `json:"cached,omitempty"`
	FileCount      int           `json:"fileCount,omitempty"`
	Paths          []string      `json:"paths,omitempty"`
	Truncated      bool          `json:"truncated,omitempty"`
	Bytes          int64         `json:"bytes,omitempty"`
	DurationMs     int64         `json:"durationMs,omitempty"`
	Error          string        `json:"error,omitempty"`
}

const maxDetailPaths = 50

// MaxSplitActivityRecords caps how many individual rows are recorded from one multi-path action.
const MaxSplitActivityRecords = maxDetailPaths

// CapPaths limits paths stored in details to avoid oversized JSON blobs.
func (d *Details) CapPaths() {
	if len(d.Paths) <= maxDetailPaths {
		return
	}
	d.Paths = d.Paths[:maxDetailPaths]
	d.Truncated = true
}

// ToFrontendDetails maps persisted details to the API shape (drops internal fields).
func (d Details) ToFrontendDetails() FrontendDetails {
	return FrontendDetails{
		TargetUsername: d.TargetUsername,
		Source:         d.Source,
		Path:           d.Path,
		TargetPath:     d.TargetPath,
		Scopes:         d.Scopes,
		AffectedTokenName: d.AffectedTokenName,
		LoginMethod:    d.LoginMethod,
		PasskeyName:    d.PasskeyName,
		UpdatedFields:  d.UpdatedFields,
		Changes:        append([]FieldChange(nil), d.Changes...),
		Cached:         d.Cached,
		FileCount:      d.FileCount,
		Paths:          append([]string(nil), d.Paths...),
		Truncated:      d.Truncated,
		Bytes:          d.Bytes,
		DurationMs:     d.DurationMs,
		Error:          d.Error,
	}
}

// Entry is a single activity log row (buffered before persistence).
type Entry struct {
	ID         int64     `json:"id,omitempty"`
	CreatedAt  int64     `json:"createdAt"`
	UserID     uint64    `json:"userId"`
	EventType  EventType `json:"eventType"`
	Source     string    `json:"source,omitempty"`
	Path       string    `json:"path,omitempty"`
	TargetPath string    `json:"targetPath,omitempty"`
	IPAddress  string    `json:"ipAddress,omitempty"`
	Details    Details   `json:"details"`
}

// FrontendEntry is the narrowed API response (like FrontendUser on User).
type FrontendEntry struct {
	ID         int64           `json:"id"`
	CreatedAt  int64           `json:"createdAt"`
	Username   string          `json:"username"`
	EventType  EventType       `json:"eventType"`
	Source     string          `json:"source,omitempty"`
	Path       string          `json:"path,omitempty"`
	TargetPath string          `json:"targetPath,omitempty"`
	ShareHash  string          `json:"shareHash,omitempty"`
	TokenName  string          `json:"tokenName,omitempty"`
	AuthMethod string          `json:"authMethod,omitempty"`
	IPAddress  string          `json:"ipAddress,omitempty"`
	Details    FrontendDetails `json:"details,omitempty"`
}

// PrepForFrontend converts a persisted entry into the API response shape.
// actorUsername is the display name for the user who performed the action (from UserID).
func (e Entry) PrepForFrontend(actorUsername string) FrontendEntry {
	fe := FrontendEntry{
		ID:         e.ID,
		CreatedAt:  e.CreatedAt,
		Username:   actorUsername,
		EventType:  e.EventType,
		Source:     e.Source,
		Path:       e.Path,
		TargetPath: e.TargetPath,
		IPAddress:  e.IPAddress,
		Details:    e.Details.ToFrontendDetails(),
	}
	if fe.Source == "" {
		fe.Source = e.Details.Source
	}
	if fe.Path == "" {
		fe.Path = e.Details.Path
	}
	if fe.TargetPath == "" {
		fe.TargetPath = e.Details.TargetPath
	}
	fe.ShareHash = e.Details.ShareHash
	if fe.TokenName == "" {
		fe.TokenName = e.Details.TokenName
	}
	if fe.AuthMethod == "" {
		fe.AuthMethod = e.Details.AuthMethod
	}
	return fe
}

// ListResponse is the paginated activity list API response.
type ListResponse struct {
	Items      []FrontendEntry `json:"items"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	TotalPages int             `json:"totalPages"`
}

// StatsBucket is one aggregated chart data point.
type StatsBucket struct {
	Bucket      int64  `json:"bucket"`
	SeriesKey   string `json:"seriesKey"`
	SeriesLabel string `json:"seriesLabel,omitempty"`
	Count       int    `json:"count"`
	EventType   string `json:"eventType,omitempty"`
}

// StatsResponse is the aggregated grouped activity API response.
type StatsResponse struct {
	Buckets []StatsBucket `json:"buckets"`
}

// GroupedResponse is an alias for chart/grouped endpoints.
type GroupedResponse = StatsResponse

// ListRow is a persisted entry with the actor username resolved from SQL.
type ListRow struct {
	Entry
	ActorUsername string
}

// QueryFilter scopes activity queries.
type QueryFilter struct {
	From       int64
	To         int64
	UserID     uint64
	UserFilter bool // when true, filter by UserID (including 0 for anonymous)
	Scope      string // all, files, shares — drives share-scope download matching
	EventTypes []EventType
	Source     string
	PathPrefix string
	PathGlob   string
	ShareHash        string
	ShareOwnerUserID uint64
	ShareOwnerFilter bool     // restrict to shares owned by ShareOwnerUserID
	OwnedShareHashes []string // legacy share-download rows without shareOwnerUserId
	Page             int
	Limit            int
	Interval   string // minute, hour, day, none — time bucket on the X-axis
	SplitBy    string // eventType, user, none — series dimension
	GroupBy    string // maps to Interval when Interval is empty
}

// MarshalDetailsJSON serializes details for SQLite storage.
func MarshalDetailsJSON(d Details) (string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

// UnmarshalDetailsJSON parses details from SQLite.
func UnmarshalDetailsJSON(raw string, d *Details) error {
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), d)
}
