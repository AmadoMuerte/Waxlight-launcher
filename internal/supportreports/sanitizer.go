package supportreports

import (
	"regexp"
	"strings"
)

var (
	sensitiveName = regexp.MustCompile(`(?i)(TOKEN|SECRET|PASSWORD|PASS|API_KEY|APIKEY|AUTH|COOKIE|CREDENTIAL|PRIVATE|SESSIONKEY|SESSIONSIGNATURE|PRELOGINTOKEN|TOTPCODE)`)
	jsonSecret    = regexp.MustCompile(`(?i)("(?:password|passwd|secret|token|api[_-]?key|authorization|cookie|credential|private[_-]?key|sessionkey|sessionsignature|prelogintoken|totp(?:code)?)"\s*:\s*")[^"]*(")`)
	plainSecret   = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key|authorization|cookie|credential|private[_-]?key|sessionkey|sessionsignature|prelogintoken|totp(?:code)?)(\s*[:=]\s*)(?:"[^"]*"|\S+)`)
	bearerSecret  = regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+`)
	linuxHome     = regexp.MustCompile(`/home/[^/\\ \t\r\n"']+`)
	macHome       = regexp.MustCompile(`/Users/[^/\\ \t\r\n"']+`)
	windowsHome   = regexp.MustCompile(`(?i)[A-Z]:\\Users\\[^\\/ \t\r\n"']+`)
	controlChars  = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
)

func SanitizeText(value string) string {
	value = linuxHome.ReplaceAllStringFunc(value, func(path string) string {
		if slash := strings.Index(path[len("/home/"):], "/"); slash >= 0 {
			return "$HOME" + path[len("/home/")+slash:]
		}
		return "$HOME"
	})
	value = windowsHome.ReplaceAllStringFunc(value, func(path string) string {
		prefix := len(`C:\Users\`)
		if slash := strings.Index(path[prefix:], `\`); slash >= 0 {
			return `%USERPROFILE%` + path[prefix+slash:]
		}
		return `%USERPROFILE%`
	})
	value = macHome.ReplaceAllStringFunc(value, func(path string) string {
		if slash := strings.Index(path[len("/Users/"):], "/"); slash >= 0 {
			return "<home>" + path[len("/Users/")+slash:]
		}
		return "<home>"
	})
	value = jsonSecret.ReplaceAllString(value, `$1<redacted>$2`)
	value = bearerSecret.ReplaceAllString(value, `$1 <redacted>`)
	value = plainSecret.ReplaceAllString(value, `$1$2<redacted>`)
	return controlChars.ReplaceAllString(value, "")
}

func SanitizeEnvironment(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for name, value := range values {
		if sensitiveName.MatchString(name) {
			result[name] = "<redacted>"
			continue
		}
		result[name] = SanitizeText(value)
	}
	return result
}

func safeSource(source string) (string, string) {
	parts := strings.Split(source, ":")
	if len(parts) >= 3 && parts[0] == "moddb" {
		return SanitizeText(parts[1]), "moddb"
	}
	return "local", "local"
}
