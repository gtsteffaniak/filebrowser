package exifcommon

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dsoprea/go-logging"
)

var (
	timeType = reflect.TypeOf(time.Time{})
)

// DumpBytes prints a list of hex-encoded bytes.
func DumpBytes(data []byte) {
	fmt.Printf("DUMP: ")
	for _, x := range data {
		fmt.Printf("%02x ", x)
	}

	fmt.Printf("\n")
}

// DumpBytesClause prints a list like DumpBytes(), but encapsulated in
// "[]byte { ... }".
func DumpBytesClause(data []byte) {
	fmt.Printf("DUMP: ")

	fmt.Printf("[]byte { ")

	for i, x := range data {
		fmt.Printf("0x%02x", x)

		if i < len(data)-1 {
			fmt.Printf(", ")
		}
	}

	fmt.Printf(" }\n")
}

// DumpBytesToString returns a stringified list of hex-encoded bytes.
func DumpBytesToString(data []byte) string {
	b := new(bytes.Buffer)

	for i, x := range data {
		_, err := b.WriteString(fmt.Sprintf("%02x", x))
		log.PanicIf(err)

		if i < len(data)-1 {
			_, err := b.WriteRune(' ')
			log.PanicIf(err)
		}
	}

	return b.String()
}

// DumpBytesClauseToString returns a comma-separated list of hex-encoded bytes.
func DumpBytesClauseToString(data []byte) string {
	b := new(bytes.Buffer)

	for i, x := range data {
		_, err := b.WriteString(fmt.Sprintf("0x%02x", x))
		log.PanicIf(err)

		if i < len(data)-1 {
			_, err := b.WriteString(", ")
			log.PanicIf(err)
		}
	}

	return b.String()
}

// ExifFullTimestampString produces a string like "2018:11:30 13:01:49" from a
// `time.Time` struct. It will attempt to convert to UTC first.
func ExifFullTimestampString(t time.Time) (fullTimestampPhrase string) {
	t = t.UTC()

	return fmt.Sprintf("%04d:%02d:%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// ParseExifFullTimestamp parses dates like "2018:11:30 13:01:49" into a UTC
// `time.Time` struct.
func ParseExifFullTimestamp(fullTimestampPhrase string) (timestamp time.Time, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	parts := strings.Split(fullTimestampPhrase, " ")
	datestampValue, timestampValue := parts[0], parts[1]

	// Normalize the separators.
	datestampValue = strings.ReplaceAll(datestampValue, "-", ":")
	timestampValue = strings.ReplaceAll(timestampValue, "-", ":")

	dateParts := strings.Split(datestampValue, ":")

	year, err := strconv.ParseUint(dateParts[0], 10, 16)
	if err != nil {
		log.Panicf("could not parse year")
	}

	month, err := strconv.ParseUint(dateParts[1], 10, 8)
	if err != nil {
		log.Panicf("could not parse month")
	}

	day, err := strconv.ParseUint(dateParts[2], 10, 8)
	if err != nil {
		log.Panicf("could not parse day")
	}

	timeParts := strings.Split(timestampValue, ":")

	hour, err := strconv.ParseUint(timeParts[0], 10, 8)
	if err != nil {
		log.Panicf("could not parse hour")
	}

	minute, err := strconv.ParseUint(timeParts[1], 10, 8)
	if err != nil {
		log.Panicf("could not parse minute")
	}

	second, err := strconv.ParseUint(timeParts[2], 10, 8)
	if err != nil {
		log.Panicf("could not parse second")
	}

	timestamp = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
	return timestamp, nil
}

// IsTime returns true if the value is a `time.Time`.
func IsTime(v interface{}) bool {

	// TODO(dustin): Add test

	return reflect.TypeOf(v) == timeType
}
