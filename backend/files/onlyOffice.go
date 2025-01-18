package files

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

var (
	OnlyOfficeCache = utils.NewCache(48*time.Hour, 1*time.Hour)
)

func getOnlyOfficeId(realpath string) string {
	// error is intentionally ignored in order treat errors
	// the same as a cache-miss
	cachedDocumentKey, ok := OnlyOfficeCache.Get(realpath).(string)
	if ok {
		return cachedDocumentKey
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	documentKey := hashSHA256(realpath + timestamp)
	OnlyOfficeCache.Set(realpath, documentKey)
	return documentKey
}

func hashSHA256(data string) string {
	bytes := sha256.Sum256([]byte(data))
	return hex.EncodeToString(bytes[:])
}
