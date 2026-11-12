// Package envvars validates user-supplied environment variable maps.
package envvars

import (
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

// InstanceDirectory is set by the launcher for every launch and cannot be
// supplied by users at either the global or instance level.
const InstanceDirectory = "WAXLIGHT_INSTANCE_DIR"

// Clean trims names and rejects invalid names, reserved variables, and values
// that cannot be passed to a process.
func Clean(values map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return map[string]string{}, nil
	}

	cleaned := make(map[string]string, len(values))
	for rawKey, value := range values {
		key := strings.TrimSpace(rawKey)
		if !validName(key) {
			return nil, errs.NewError(errs.ErrValidation, "Environment variable names must use letters, numbers, and underscores and cannot start with a number")
		}
		if key == InstanceDirectory {
			return nil, errs.NewError(errs.ErrValidation, InstanceDirectory+" is reserved by Waxlight")
		}
		if strings.ContainsRune(value, '\x00') {
			return nil, errs.NewError(errs.ErrValidation, "Environment variable values cannot contain NUL characters")
		}
		cleaned[key] = value
	}
	return cleaned, nil
}

func validName(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
