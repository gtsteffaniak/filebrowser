package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// accessGetHandler lists all access rules or retrieves a specific rule.
// @Summary List access rules
// @Description Lists access rules. Can be filtered by source, path, user, or group. Can also be grouped by user or group.
// @Tags Access
// @Accept json
// @Produce json
// @Param source query string false "Source name (e.g. 'default')"
// @Param path query string false "Index path (e.g. /secret)"
// @Param user query string false "Username to filter rules for"
// @Param group query string false "Group name to filter rules for"
// @Param groupBy query string false "Group results by 'user' or 'group'"
// @Success 200 {object} object "Varies based on query. Can be access.FrontendAccessRule, []access.PrincipalRule, map[string][]access.PrincipalRule, or map[string]access.FrontendAccessRule"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [get]
func accessGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourceName := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")
	user := r.URL.Query().Get("user")
	group := r.URL.Query().Get("group")
	groupBy := r.URL.Query().Get("groupBy")

	var sourcePath string
	if sourceName != "" {
		index := indexing.GetIndex(sourceName)
		if index == nil {
			return http.StatusBadRequest, fmt.Errorf("source not found: %s", sourceName)
		}
		sourcePath = index.Path
	}

	// Return rules based on input parameters
	if user != "" {
		rules := store.Access.GetRulesForUser(sourcePath, user)
		return renderJSON(w, r, rules)
	}
	if group != "" {
		rules := store.Access.GetRulesForGroup(sourcePath, group)
		return renderJSON(w, r, rules)
	}
	if groupBy == "user" {
		rules := store.Access.GetAllRulesByUsers(sourcePath)
		return renderJSON(w, r, rules)
	}
	if groupBy == "group" {
		rules := store.Access.GetAllRulesByGroups(sourcePath)
		return renderJSON(w, r, rules)
	}

	if sourceName == "" {
		return http.StatusBadRequest, fmt.Errorf("source parameter is required for this query")
	}

	if indexPath != "" {
		rule, _ := store.Access.GetFrontendRules(sourcePath, indexPath)
		return renderJSON(w, r, rule)
	}

	// List all rules for the source by default
	all, err := store.Access.GetAllRules(sourcePath)
	if err != nil {
		logger.Errorf("failed to retrieve access rules: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to retrieve access rules: %w", err)
	}
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
// @Param body body object{allow=bool,ruleCategory=string,value=string} true "Rule details: allow (true/false), ruleCategory (user/group), value (username or groupname)"
// @Success 200 {object} map[string]string "Rule added or updated"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [post]
func accessPostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourceName := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")
	index := indexing.GetIndex(sourceName)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("source not found: %s", sourceName)
	}
	var body struct {
		Allow        bool   `json:"allow"`
		RuleCategory string `json:"ruleCategory"`
		Value        string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err)
	}

	if indexPath == "" || body.RuleCategory == "" || body.Value == "" {
		return http.StatusBadRequest, fmt.Errorf("all parameters (path, ruleCategory, value) are required")
	}
	var err error
	if body.Allow {
		switch body.RuleCategory {
		case "user":
			err = store.Access.AllowUser(index.Path, indexPath, body.Value)
		case "group":
			err = store.Access.AllowGroup(index.Path, indexPath, body.Value)
		default:
			return http.StatusBadRequest, fmt.Errorf("invalid ruleCategory: must be 'user' or 'group'")
		}
	} else {
		switch body.RuleCategory {
		case "user":
			err = store.Access.DenyUser(index.Path, indexPath, body.Value)
		case "group":
			err = store.Access.DenyGroup(index.Path, indexPath, body.Value)
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
// @Description Delete a user or group from an allow or deny list for a sourcePath and indexPath.
// @Tags Access
// @Accept json
// @Produce json
// @Param source query string true "Source path prefix (e.g. mnt/storage)"
// @Param path query string true "Index path (e.g. /secret)"
// @Param ruleType query string true "Rule type (allow or deny)"
// @Param ruleCategory query string true "Rule category (user or group)"
// @Param value query string true "Username or groupname to remove"
// @Success 200 {object} map[string]string "Rule entry deleted"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [delete]
func accessDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourceName := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")
	index := indexing.GetIndex(sourceName)
	if index == nil {
		return 500, fmt.Errorf("source not found: %s", sourceName)
	}

	var body struct {
		Allow        bool   `json:"allow"`
		RuleCategory string `json:"ruleCategory"`
		Value        string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err)
	}

	if indexPath == "" || body.RuleCategory == "" || body.Value == "" {
		return http.StatusBadRequest, fmt.Errorf("all parameters (path, ruleCategory, value) are required")
	}

	var found bool
	var err error
	if body.Allow {
		switch body.RuleCategory {
		case "user":
			found, err = store.Access.RemoveAllowUser(index.Path, indexPath, body.Value)
		case "group":
			found, err = store.Access.RemoveAllowGroup(index.Path, indexPath, body.Value)
		}
	} else {
		switch body.RuleCategory {
		case "user":
			found, err = store.Access.RemoveDenyUser(index.Path, indexPath, body.Value)
		case "group":
			found, err = store.Access.RemoveDenyGroup(index.Path, indexPath, body.Value)
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
