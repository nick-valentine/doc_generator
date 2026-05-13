package generators

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed i18n/*.json
var i18nFS embed.FS

// uiTranslations dynamically loads the localized strings for dashboards.
var uiTranslations = map[string]map[string]string{}

func init() {
	entries, err := i18nFS.ReadDir("i18n")
	if err != nil {
		panic(fmt.Sprintf("failed to read i18n embedded dir: %v", err))
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			lang := strings.TrimSuffix(name, ".json")

			data, err := i18nFS.ReadFile("i18n/" + name)
			if err != nil {
				panic(fmt.Sprintf("failed to read embedded i18n file %s: %v", name, err))
			}

			var dict map[string]string
			if err := json.Unmarshal(data, &dict); err != nil {
				panic(fmt.Sprintf("failed to unmarshal i18n file %s: %v", name, err))
			}
			uiTranslations[lang] = dict
		}
	}
}

// Translate returns the localized string for the specified locale and key.
// Defaults to English if the locale or the key is not defined.
func Translate(locale, key string) string {
	l := strings.ToLower(locale)
	// If locale isn't provided or is not standard, default to English
	if l != "en" && l != "ja" {
		l = "en"
	}

	dict, exists := uiTranslations[l]
	if !exists {
		dict = uiTranslations["en"]
	}

	val, exists := dict[key]
	if !exists {
		// Fallback to English dict
		val, exists = uiTranslations["en"][key]
		if !exists {
			return key
		}
	}
	return val
}
