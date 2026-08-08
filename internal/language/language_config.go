package language

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	languagepkg "golang.org/x/text/language"
)

//go:embed languages.json
var languageConfigJSON []byte

type Language struct {
	Code       string   `json:"code"`
	NativeName string   `json:"nativeName"`
	Aliases    []string `json:"aliases"`
}

type LanguageConfig struct {
	DefaultLanguage string     `json:"defaultLanguage"`
	Languages       []Language `json:"languages"`
	known           map[string]struct{}
	aliases         map[string]string
}

var (
	languageConfigOnce  sync.Once
	languageErrorLogged sync.Once
	languageConfig      LanguageConfig
	languageConfigErr   error
)

func Languages() (LanguageConfig, error) {
	languageConfigOnce.Do(loadLanguages)
	languageErrorLogged.Do(func() {
		if languageConfigErr != nil {
			slog.Error("language configuration is broken", "error", languageConfigErr)
		}
	})
	return languageConfig, languageConfigErr
}

func NormalizeLanguage(value string) string {
	config, err := Languages()
	if err != nil {
		return "en"
	}
	return config.Normalize(value)
}

func (config LanguageConfig) Normalize(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "_", "-")
	legacyCode, _, _ := strings.Cut(strings.ToLower(value), "-")
	if normalized, ok := config.aliases[legacyCode]; ok {
		return normalized
	}
	tag, err := languagepkg.Parse(value)
	if err != nil {
		return config.DefaultLanguage
	}
	base, _ := tag.Base()
	if _, supported := config.known[base.String()]; supported {
		return base.String()
	}
	return config.DefaultLanguage
}

func loadLanguages() {
	if err := json.Unmarshal(languageConfigJSON, &languageConfig); err != nil {
		languageConfigErr = fmt.Errorf("parse languages.json: %w", err)
		return
	}
	languageConfig.known = make(map[string]struct{}, len(languageConfig.Languages))
	languageConfig.aliases = make(map[string]string)
	for _, language := range languageConfig.Languages {
		if language.Code == "" || language.NativeName == "" {
			languageConfigErr = fmt.Errorf("language code and native name are required")
			return
		}
		base, err := languagepkg.Parse(language.Code)
		if err != nil {
			languageConfigErr = fmt.Errorf("parse language code %q: %w", language.Code, err)
			return
		}
		code, _ := base.Base()
		if code.String() != language.Code {
			languageConfigErr = fmt.Errorf("language code %q must be a base BCP 47 code", language.Code)
			return
		}
		if _, duplicate := languageConfig.known[language.Code]; duplicate {
			languageConfigErr = fmt.Errorf("duplicate language code %q", language.Code)
			return
		}
		languageConfig.known[language.Code] = struct{}{}
		for _, alias := range language.Aliases {
			alias = strings.ToLower(strings.TrimSpace(alias))
			if alias == "" {
				languageConfigErr = fmt.Errorf("empty alias for language %q", language.Code)
				return
			}
			if _, duplicate := languageConfig.known[alias]; duplicate {
				languageConfigErr = fmt.Errorf("alias duplicates language code %q", alias)
				return
			}
			if _, duplicate := languageConfig.aliases[alias]; duplicate {
				languageConfigErr = fmt.Errorf("duplicate language alias %q", alias)
				return
			}
			languageConfig.aliases[alias] = language.Code
		}
	}
	if _, supported := languageConfig.known[languageConfig.DefaultLanguage]; !supported {
		languageConfigErr = fmt.Errorf("default language %q is not configured", languageConfig.DefaultLanguage)
	}
}
