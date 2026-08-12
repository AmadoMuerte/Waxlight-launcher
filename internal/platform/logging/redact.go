package logging

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

// sensitiveKeys are field names whose values are masked before a log line is
// stored or exported. Matching is case-insensitive.
var sensitiveKeys = []string{
	"password", "passwd", "pwd", "secret",
	"token", "sessionkey", "sessionsignature", "prelogintoken",
	"totpcode", "totp", "apikey", "api_key", "authorization",
	"useremail", "email", "emailaddress",
}

var (
	redactOnce    sync.Once
	keyValueJSON  *regexp.Regexp
	keyValueToken *regexp.Regexp
	bearerToken   *regexp.Regexp
	keyName       *regexp.Regexp
	base64Long    *regexp.Regexp
)

func compileRedaction() {
	keyPattern := "(?i)\\b(?:" + strings.Join(escapeAll(sensitiveKeys), "|") + ")"
	keyValueJSON = regexp.MustCompile(`("` + keyPattern + `"\s*:\s*")[^"]*(")`)
	keyValueToken = regexp.MustCompile(`(\b` + keyPattern + `)(\s*[:=]\s*)(?:"([^"]*)"|(\S+))`)
	bearerToken = regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+([A-Za-z0-9._~+/=-]{8,})`)
	keyName = regexp.MustCompile(keyPattern)
	base64Long = regexp.MustCompile(`[A-Za-z0-9+/]{24,}={0,2}`)
}

func escapeAll(keys []string) []string {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, regexp.QuoteMeta(key))
	}
	return result
}

// Redact masks credential-like values so sensitive data never reaches the
// console, the in-memory buffer, or an exported log file. It recognizes JSON
// key/value pairs, plain key=value assignments, bearer-style tokens, and long
// base64 blobs.
func Redact(text string) string {
	redactOnce.Do(compileRedaction)
	text = keyValueJSON.ReplaceAllString(text, `$1***$2`)
	text = bearerToken.ReplaceAllString(text, `${1} ***`)
	text = keyValueToken.ReplaceAllString(text, `${1}${2}***`)
	// Mask standalone long base64 blobs that are not already masked.
	text = base64Long.ReplaceAllStringFunc(text, func(candidate string) string {
		if strings.HasPrefix(candidate, "***") {
			return candidate
		}
		return "***"
	})
	return text
}

// containsSensitiveKey reports whether the text names a sensitive field. It is
// used by the export builder to double check summary sections.
func containsSensitiveKey(text string) bool {
	redactOnce.Do(compileRedaction)
	return keyName.MatchString(text)
}

func logRedactedLine(entry Entry) {
	fmt.Fprintln(os.Stderr, entry.Line())
}
