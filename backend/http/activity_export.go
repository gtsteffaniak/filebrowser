package http

import (
	"fmt"
	"strings"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
)

var activityOptionalExportColumns = map[string]struct{}{
	"source":    {},
	"path":      {},
	"shareHash": {},
	"tokenName": {},
}

func parseActivityExportRows(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if _, ok := activityOptionalExportColumns[key]; !ok {
			return nil, fmt.Errorf("invalid rows column: %s (allowed: source, path, shareHash, tokenName)", key)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out, nil
}

func activityExportHeader(includeDetails bool, optional []string) []string {
	header := []string{"id", "createdAt", "username", "eventType"}
	header = append(header, optional...)
	header = append(header, "ipAddress", "status")
	if includeDetails {
		header = append(header, "details")
	}
	return header
}

func activityExportRowValues(item activitydb.FrontendEntry, optional []string, includeDetails bool, detailsJSON string) []string {
	row := []string{
		fmt.Sprintf("%d", item.ID),
		fmt.Sprintf("%d", item.CreatedAt),
		item.Username,
		string(item.EventType),
	}
	for _, col := range optional {
		switch col {
		case "source":
			row = append(row, item.Source)
		case "path":
			row = append(row, item.Path)
		case "shareHash":
			row = append(row, item.ShareHash)
		case "tokenName":
			row = append(row, item.TokenName)
		}
	}
	row = append(row, item.IPAddress, fmt.Sprintf("%d", item.Status))
	if includeDetails {
		row = append(row, detailsJSON)
	}
	return row
}
