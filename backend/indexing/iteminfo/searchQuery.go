package iteminfo

import (
	"regexp"
	"strings"
	"time"
)

var typeRegexp = regexp.MustCompile(`type:(\S+)`)

type SearchOptions struct {
	Conditions  map[string]bool
	LargerThan  int
	SmallerThan int
	// ModifiedOlderThan is Unix seconds (UTC start of day). If > 0, item must have ModTime strictly before this instant.
	ModifiedOlderThan int64
	// ModifiedNewerThan is Unix seconds (UTC start of day). If > 0, item must have ModTime >= this instant.
	ModifiedNewerThan int64
	Terms             []string
}

func ParseSearch(value string) SearchOptions {
	opts := SearchOptions{
		Conditions: map[string]bool{
			"exact": strings.Contains(value, "case:exact"),
		},
		Terms: []string{},
	}

	// removes the options from the value
	value = strings.Replace(value, "case:exact", "", -1)
	value = strings.TrimSpace(value)

	types := typeRegexp.FindAllStringSubmatch(value, -1)
	for _, filterType := range types {
		if len(filterType) == 1 {
			continue
		}
		filter := filterType[1]
		switch filter {
		case "image":
			opts.Conditions["image"] = true
		case "audio", "music":
			opts.Conditions["audio"] = true
		case "video":
			opts.Conditions["video"] = true
		case "doc":
			opts.Conditions["doc"] = true
		case "archive":
			opts.Conditions["archive"] = true
		case "folder":
			opts.Conditions["dir"] = true
		case "file":
			opts.Conditions["dir"] = false
		}
		if len(filter) < 8 {
			continue
		}
		if strings.HasPrefix(filter, "largerThan=") {
			opts.Conditions["larger"] = true
			size := strings.TrimPrefix(filter, "largerThan=")
			opts.LargerThan = UpdateSize(size)
		}
		if strings.HasPrefix(filter, "smallerThan=") {
			opts.Conditions["smaller"] = true
			size := strings.TrimPrefix(filter, "smallerThan=")
			opts.SmallerThan = UpdateSize(size)
		}
		if strings.HasPrefix(filter, "olderThan=") {
			dateStr := strings.TrimPrefix(filter, "olderThan=")
			if ts, ok := parseSearchDateUTCStart(dateStr); ok {
				opts.ModifiedOlderThan = ts
			}
		}
		if strings.HasPrefix(filter, "newerThan=") {
			dateStr := strings.TrimPrefix(filter, "newerThan=")
			if ts, ok := parseSearchDateUTCStart(dateStr); ok {
				opts.ModifiedNewerThan = ts
			}
		}
	}

	if len(types) > 0 {
		// Remove the fields from the search value
		value = typeRegexp.ReplaceAllString(value, "")
	}

	if value == "" {
		return opts
	}

	// if the value starts with " and finishes what that character, we will
	// only search for that term
	if value[0] == '"' && value[len(value)-1] == '"' {
		unique := strings.TrimPrefix(value, "\"")
		unique = strings.TrimSuffix(unique, "\"")

		opts.Terms = []string{unique}
		return opts
	}
	value = strings.TrimSpace(value)
	opts.Terms = strings.Split(value, "|")
	return opts
}

// parseSearchDateUTCStart parses YYYY-MM-DD as midnight UTC; used for search date filters.
func parseSearchDateUTCStart(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return 0, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return 0, false
	}
	return t.Unix(), true
}
