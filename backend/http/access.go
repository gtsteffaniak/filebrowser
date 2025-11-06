package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

type GroupListResponse struct {
	Groups []string `json:"groups"`
}

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
// @Success 200 {object} object "Varies based on query. Can be access.FrontendAccessRule (when source and path specified), []access.PrincipalRule, map[string][]access.PrincipalRule, or map[string]access.FrontendAccessRule"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [get]
func accessGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourceName := r.URL.Query().Get("source")
	indexPath := r.URL.Query().Get("path")
	user := r.URL.Query().Get("user")
	group := r.URL.Query().Get("group")

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

	if indexPath == "" || body.RuleCategory == "" || (body.RuleCategory != "all" && body.Value == "") {
		return http.StatusBadRequest, fmt.Errorf("path, ruleCategory, and value are required, unless ruleCategory is 'all'")
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
		case "all":
			err = store.Access.DenyAll(index.Path, indexPath)
		default:
			return http.StatusBadRequest, fmt.Errorf("invalid ruleCategory: must be 'user', 'group', or 'all'")
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
// @Description Delete a user or group from an allow or deny list for a sourcePath and indexPath. When cascade=true, removes the user/group from the specified path and all subpaths.
// @Tags Access
// @Accept json
// @Produce json
// @Param source query string true "Source path prefix (e.g. mnt/storage)"
// @Param path query string true "Index path (e.g. /secret)"
// @Param ruleType query string true "Rule type (allow or deny)"
// @Param ruleCategory query string true "Rule category (user or group)"
// @Param value query string true "Username or groupname to remove"
// @Param cascade query boolean false "Cascade delete to all subpaths (default: false)"
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

	ruleType := r.URL.Query().Get("ruleType")
	ruleCategory := r.URL.Query().Get("ruleCategory")
	value := r.URL.Query().Get("value")
	cascade := r.URL.Query().Get("cascade") == "true"
	allow := ruleType == "allow"

	if indexPath == "" || ruleCategory == "" || (ruleCategory != "all" && value == "") {
		return http.StatusBadRequest, fmt.Errorf("path, ruleCategory, and value are required, unless ruleCategory is 'all'")
	}

	// Handle cascade delete
	if cascade {
		if ruleCategory == "all" {
			return http.StatusBadRequest, fmt.Errorf("cascade delete is not supported for 'all' rule category")
		}

		var count int
		var err error

		switch ruleCategory {
		case "user":
			count, err = store.Access.RemoveUserCascade(index.Path, indexPath, value, allow)
		case "group":
			count, err = store.Access.RemoveGroupCascade(index.Path, indexPath, value, allow)
		default:
			return http.StatusBadRequest, fmt.Errorf("invalid ruleCategory for cascade delete: must be 'user' or 'group'")
		}

		if err != nil {
			logger.Errorf("failed to cascade delete rule entry: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to cascade delete rule entry: %w", err)
		}

		if count == 0 {
			return http.StatusNotFound, fmt.Errorf("no entries found in rule hierarchy")
		}

		return renderJSON(w, r, map[string]interface{}{
			"message": "rule entries deleted",
			"count":   count,
		})
	}

	// Handle non-cascade delete (original behavior)
	var found bool
	var err error
	if allow {
		switch ruleCategory {
		case "user":
			found, err = store.Access.RemoveAllowUser(index.Path, indexPath, value)
		case "group":
			found, err = store.Access.RemoveAllowGroup(index.Path, indexPath, value)
		}
	} else {
		switch ruleCategory {
		case "user":
			found, err = store.Access.RemoveDenyUser(index.Path, indexPath, value)
		case "group":
			found, err = store.Access.RemoveDenyGroup(index.Path, indexPath, value)
		case "all":
			found, err = store.Access.RemoveDenyAll(index.Path, indexPath)
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

// groupGetHandler retrieves all groups or groups for a specific user.
// @Summary Get all groups or groups for a user
// @Description Returns a list of all groups or the groups for a specific user.
// @Tags Access
// @Accept json
// @Produce json
// @Param user query string false "User name"
// @Success 200 {object} GroupListResponse "Object containing a list of groups"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/access/groups [get]
func groupGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, nil
	}
	user := r.URL.Query().Get("user")
	if user != "" {
		groups := store.Access.GetUserGroups(user)
		return renderJSON(w, r, &GroupListResponse{Groups: groups})
	}
	groups := store.Access.GetAllGroups()
	return renderJSON(w, r, &GroupListResponse{Groups: groups})
}

// groupPostHandler adds a user to a group.
// @Summary Add a user to a group
// @Description Adds a user to a group.
// @Tags Access
// @Accept json
// @Produce json
// @Param group query string true "Group name"
// @Param user query string true "User name"
// @Success 200 "User added to group successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/access/group [post]
func groupPostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, nil
	}
	group := r.URL.Query().Get("group")
	user := r.URL.Query().Get("user")
	err := store.Access.AddUserToGroup(group, user)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// groupDeleteHandler removes a user from a group.
// @Summary Remove a user from a group
// @Description Removes a user from a group.
// @Tags Access
// @Accept json
// @Produce json
// @Param group query string true "Group name"
// @Param user query string true "User name"
// @Success 200 "User removed from group successfully"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/access/group [delete]
func groupDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, nil
	}
	group := r.URL.Query().Get("group")
	user := r.URL.Query().Get("user")
	err := store.Access.RemoveUserFromGroup(group, user)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// accessPatchHandler updates an access rule's path.
// @Summary Update access rule path
// @Description Updates the path for a specific access rule
// @Tags Access
// @Accept json
// @Produce json
// @Param body body object{source=string,oldPath=string,newPath=string} true "Source, old path, and new path"
// @Success 200 {object} map[string]string "Rule path updated successfully"
// @Failure 400 {object} map[string]string "Bad request - missing or invalid parameters"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/access [patch]
func accessPatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var body struct {
		Source  string `json:"source"`
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
	}
	defer r.Body.Close()

	if body.Source == "" || body.OldPath == "" || body.NewPath == "" {
		return http.StatusBadRequest, fmt.Errorf("source, oldPath, and newPath are required")
	}

	// Get the index for the source
	index := indexing.GetIndex(body.Source)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("source not found: %s", body.Source)
	}

	// Update the access rule path
	err := store.Access.UpdateRulePath(index.Path, body.OldPath, body.NewPath)
	if err != nil {
		logger.Errorf("failed to update rule path: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to update rule path: %w", err)
	}

	return renderJSON(w, r, map[string]string{"message": "rule path updated"})
}
