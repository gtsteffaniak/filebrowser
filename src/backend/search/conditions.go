package search

import (
	"regexp"
	"strings"
)

var typeRegexp = regexp.MustCompile(`type:(\w+)`)

var documentTypes = []string{
	".word",
	".pdf",
	".txt",
}

var compressedFile = []string{
	".7z",
	".rar",
	".zip",
	".tar",
	".tar.gz",
	".tar.xz",
}

type searchOptions struct {
	Conditions    map[string]bool
	Terms         []string
}

func ParseSearch(value string) *searchOptions {
	opts := &searchOptions{
		Conditions:    map[string]bool{
			"exact": strings.Contains(value, "case:exact"),
		},
		Terms:         []string{},
	}

	// removes the options from the value
	value = strings.Replace(value, "case:exact", "", -1)
	value = strings.Replace(value, "case:exact", "", -1)
	value = strings.TrimSpace(value)

	types := typeRegexp.FindAllStringSubmatch(value, -1)
	for _, filterType := range types {
		if len(filterType) == 1 {
			continue
		}
		
		switch filterType[1] {
			case "image":
				opts.Conditions["image"] = true
			case "audio", "music":
				opts.Conditions["audio"] = true
			case "video":
				opts.Conditions["video"] = true
			case "doc":
				opts.Conditions["doc"] = true
			case "zip":
				opts.Conditions["zip"] = true
		}
	}

	if len(types) > 0 {
		// Remove the fields from the search value.
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

	opts.Terms = strings.Split(value, " ")
	return opts
}
