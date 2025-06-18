package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/go-logger/logger"
)

// accessGetHandler lists all access rules or retrieves a specific rule.
// @Summary List access rules
// @Description List all access rules or retrieve a specific rule by sourcePath and indexPath.
// @Tags Access
// @Accept json
// @Produce json
// @Param sourcePath query string false "Source path prefix (e.g. mnt/storage)"
// @Param indexPath query string false "Index path (e.g. /secret)"
// @Success 200 {object} access.AccessRule "List of access rules or specific rule details"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [get]
func accessGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourcePath := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")

	if sourcePath != "" && indexPath != "" {
		rule, ok := store.Access.GetRule(sourcePath, indexPath)
		if !ok {
			return http.StatusNotFound, fmt.Errorf("rule not found")
		}
		return renderJSON(w, r, rule)
	}

	// List all rules
	all := store.Access.GetAllRules()
	return renderJSON(w, r, all)
}

// accessPostHandler adds or updates an access rule.
// @Summary Add or update access rule
// @Description Add or update an access rule for a sourcePath and indexPath.
// @Tags Access
// @Accept json
// @Produce json
// @Param source query string true "Source path prefix (e.g. mnt/storage)"
// @Param path query string true "Index path (e.g. /secret)"
// @Param body body object{whitelist=bool,ruleCategory=string,value=string} true "Rule details: whitelist (true/false), ruleCategory (user/group), value (username or groupname)"
// @Success 200 {object} map[string]string "Rule added or updated"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [post]
func accessPostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourcePath := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")

	var body struct {
		Whitelist    bool   `json:"whitelist"`
		RuleCategory string `json:"ruleCategory"`
		Value        string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err)
	}

	if sourcePath == "" || indexPath == "" || body.RuleCategory == "" || body.Value == "" {
		return http.StatusBadRequest, fmt.Errorf("all parameters (source, path, ruleCategory, value) are required")
	}
	var err error
	if body.Whitelist {
		switch body.RuleCategory {
		case "user":
			err = store.Access.WhitelistUser(sourcePath, indexPath, body.Value)
		case "group":
			err = store.Access.WhitelistGroup(sourcePath, indexPath, body.Value)
		default:
			return http.StatusBadRequest, fmt.Errorf("invalid ruleCategory: must be 'user' or 'group'")
		}
	} else {
		switch body.RuleCategory {
		case "user":
			err = store.Access.BlacklistUser(sourcePath, indexPath, body.Value)
		case "group":
			err = store.Access.BlacklistGroup(sourcePath, indexPath, body.Value)
		default:
			return http.StatusBadRequest, fmt.Errorf("invalid ruleCategory: must be 'user' or 'group'")
		}
	}
	if err != nil {
		logger.Errorf("failed to add or update rule: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to add or update rule: %w", err)
	}
	return renderJSON(w, r, map[string]string{"message": "rule added or updated"})
}

// accessDeleteHandler deletes a single user or group from a rule.
// @Summary Delete access rule entry
// @Description Delete a user or group from a whitelist or blacklist for a sourcePath and indexPath.
// @Tags Access
// @Accept json
// @Produce json
// @Param source query string true "Source path prefix (e.g. mnt/storage)"
// @Param path query string true "Index path (e.g. /secret)"
// @Param ruleType query string true "Rule type (whitelist or blacklist)"
// @Param ruleCategory query string true "Rule category (user or group)"
// @Param value query string true "Username or groupname to remove"
// @Success 200 {object} map[string]string "Rule entry deleted"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [delete]
func accessDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourcePath := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")
	ruleType := r.URL.Query().Get("ruleType")         // whitelist or blacklist
	ruleCategory := r.URL.Query().Get("ruleCategory") // user or group
	value := r.URL.Query().Get("value")               // username or groupname

	if sourcePath == "" || indexPath == "" || ruleType == "" || ruleCategory == "" || value == "" {
		return http.StatusBadRequest, fmt.Errorf("all parameters (source, path, ruleType, ruleCategory, value) are required")
	}

	var found bool
	var err error
	switch ruleType {
	case "whitelist":
		switch ruleCategory {
		case "user":
			found, err = store.Access.RemoveWhitelistUser(sourcePath, indexPath, value)
		case "group":
			found, err = store.Access.RemoveWhitelistGroup(sourcePath, indexPath, value)
		}
	case "blacklist":
		switch ruleCategory {
		case "user":
			found, err = store.Access.RemoveBlacklistUser(sourcePath, indexPath, value)
		case "group":
			found, err = store.Access.RemoveBlacklistGroup(sourcePath, indexPath, value)
		}
	}
	if !found {
		if err != nil {
			logger.Errorf("failed to remove rule entry: %v", err)
		}
		return http.StatusNotFound, fmt.Errorf("entry not found in rule")
	}

	return renderJSON(w, r, map[string]string{"message": "rule entry deleted"})
}
