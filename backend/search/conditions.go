package search

import (
	"regexp"
	"strings"
	"strconv"
)

var typeRegexp = regexp.MustCompile(`type:(\S+)`)

var documentTypes = []string{
	".word",
	".pdf",
	".txt",
	".doc",
	".docx",
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
	Size 			int
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
		filter := filterType[1]
		switch filter {
			case "image"			: opts.Conditions["image"] 		= true
			case "audio", "music"	: opts.Conditions["audio"] 		= true
			case "video"			: opts.Conditions["video"] 		= true
			case "doc"				: opts.Conditions["doc"] 		= true
			case "archive"			: opts.Conditions["archive"] 	= true
			case "folder"			: opts.Conditions["dir"] 		= true
			case "file"				: opts.Conditions["dir"] 		= false
		}
		if len(filter) < 8 {
			continue
		}
		if filter[:7] == "larger=" {
			opts.Conditions["larger"] = true
			opts.Size = updateSize(filter[7:]) // everything after larger=
		}
		if filter[:8] == "smaller=" {
			opts.Conditions["smaller"] = true
			opts.Size = updateSize(filter[8:]) // everything after smaller=
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

func toInt(str string) int {
	val, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return val
}

func updateSize(given string) int {
	size := toInt(given)
	if size == 0 {
		return 100
	} else {
		return size
	}
}