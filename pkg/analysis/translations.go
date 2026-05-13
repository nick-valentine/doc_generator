package analysis

import (
	"bufio"
	"doc_generator/pkg/store"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Regex pattern matching typical base ISO 639-1 codes, e.g., "en", "fr_FR", "pt-BR", "zh_Hans"
var localeFilenameRegex = regexp.MustCompile(`(?i)(?:^|[_.\-/])([a-z]{2}(?:[_-][A-Za-z]{2,4})?)\.(json|properties|po|pot)$`)

// System files to explicitly ignore to prevent loading massive config bundles
var ignoreList = map[string]bool{
	"package.json":       true,
	"package-lock.json":  true,
	"tsconfig.json":      true,
	"jsconfig.json":      true,
	"composer.json":      true,
	"composer.lock":      true,
	"yarn.lock":          true,
	"manifest.json":      true,
	"gradle.properties":  true,
	"gradle-daemon-jvm.properties": true,
	"gradle-wrapper.properties":    true,
}

// RunTranslationAnalysis crawls the target root path for global translation assets
// (JSON, Properties, and GNU PO files), extracting localized string matrices.
func RunTranslationAnalysis(source *store.Source, rootDir string) {
	_ = filepath.WalkDir(rootDir, func(fPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			// Skip system and vendor folders to match main.go walking limits
			if (strings.HasPrefix(name, ".") && name != ".") || name == "node_modules" || name == "vendor" || name == "venv" {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if ignoreList[strings.ToLower(name)] {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".properties" && ext != ".po" && ext != ".pot" {
			return nil
		}

		// Perform heuristic to determine if it's a localized resource.
		// Case 1: Resides in localized naming folders (i18n, locales, lang, translations)
		lowerPath := strings.ToLower(fPath)
		isLocFolder := strings.Contains(lowerPath, "/i18n/") || 
			strings.Contains(lowerPath, "/locale/") || 
			strings.Contains(lowerPath, "/locales/") || 
			strings.Contains(lowerPath, "/lang/") || 
			strings.Contains(lowerPath, "/translations/")

		// Case 2: Strong locale match in filename (e.g., messages_en.properties or fr.json)
		localeMatch := localeFilenameRegex.FindStringSubmatch(name)
		
		// Require either a locale folder parent OR an explicit locale code match in the filename
		if !isLocFolder && len(localeMatch) == 0 {
			return nil
		}

		// Extract explicit locale code or default to reasonable inference
		locale := "unknown"
		if len(localeMatch) >= 2 {
			locale = localeMatch[1]
			// Strip out standard word stems that may trick the regex (e.g. "build.json" yielding "ld")
			if len(locale) <= 1 {
				locale = "unknown"
			}
		}

		// Read file content
		content, err := os.ReadFile(fPath)
		if err != nil {
			return nil
		}

		// Handle specific parser mappings
		switch ext {
		case ".json":
			parseJSONTranslations(source, fPath, content, locale)
		case ".properties":
			parsePropertiesTranslations(source, fPath, content, locale)
		case ".po", ".pot":
			parsePOTranslations(source, fPath, content, locale)
		}

		return nil
	})
}

// parseJSONTranslations iterates nested structures and flattens string keys
func parseJSONTranslations(source *store.Source, filePath string, data []byte, defaultLocale string) {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	// Detect custom locale if set inside json (optional standard fallback)
	locale := defaultLocale
	if locale == "unknown" {
		// If we couldn't extract from name, check for "locale" or "lang" top-level key
		if m, ok := raw.(map[string]interface{}); ok {
			if l, ok := m["locale"].(string); ok {
				locale = l
			} else if l, ok := m["lang"].(string); ok {
				locale = l
			}
		}
	}

	// Perform flattening walk
	flattenJSON(source, filePath, locale, "", raw)
}

func flattenJSON(source *store.Source, filePath, locale, prefix string, val interface{}) {
	switch v := val.(type) {
	case string:
		if prefix != "" && locale != "unknown" && v != "" {
			source.Translations = append(source.Translations, store.TranslationRecord{
				Key:    prefix,
				Locale: locale,
				Value:  v,
				File:   filePath,
				Format: "json",
			})
		}
	case map[string]interface{}:
		for k, item := range v {
			newKey := k
			if prefix != "" {
				newKey = prefix + "." + k
			}
			flattenJSON(source, filePath, locale, newKey, item)
		}
	}
}

// parsePropertiesTranslations reads typical java messages bundles
func parsePropertiesTranslations(source *store.Source, filePath string, data []byte, defaultLocale string) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	locale := defaultLocale

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore empty lines and comments (# or !)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		// Split by '=' or ':' (standard properties delimiters)
		idx := strings.IndexAny(line, "=:")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Clean up any trailing/leading quotes if present
		val = strings.Trim(val, "\"")

		if key != "" && val != "" && locale != "unknown" {
			source.Translations = append(source.Translations, store.TranslationRecord{
				Key:    key,
				Locale: locale,
				Value:  val,
				File:   filePath,
				Format: "properties",
			})
		}
	}
}

// parsePOTranslations reads GNU translation blocks: msgid and msgstr tokens
func parsePOTranslations(source *store.Source, filePath string, data []byte, defaultLocale string) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	locale := defaultLocale

	// Compile scanner regexes for PO blocks
	msgidRegex := regexp.MustCompile(`^msgid\s+"(.*)"\s*$`)
	msgstrRegex := regexp.MustCompile(`^msgstr\s+"(.*)"\s*$`)
	metaLocaleRegex := regexp.MustCompile(`(?i)"Language:\s*([a-zA-Z_-]+)\\n"`)

	var currentKey string
	var inMsgid bool
	var inMsgstr bool
	var accumulatedStr strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try to extract meta-locale directly from headers
		if locale == "unknown" {
			if metaMatch := metaLocaleRegex.FindStringSubmatch(line); len(metaMatch) >= 2 {
				locale = metaMatch[1]
			}
		}

		if matches := msgidRegex.FindStringSubmatch(line); len(matches) >= 2 {
			// Handle flushing the previous completed translation
			if currentKey != "" && accumulatedStr.Len() > 0 && locale != "unknown" {
				source.Translations = append(source.Translations, store.TranslationRecord{
					Key:    currentKey,
					Locale: locale,
					Value:  accumulatedStr.String(),
					File:   filePath,
					Format: "po",
				})
			}
			
			currentKey = matches[1]
			accumulatedStr.Reset()
			inMsgid = true
			inMsgstr = false
			continue
		}

		if matches := msgstrRegex.FindStringSubmatch(line); len(matches) >= 2 {
			accumulatedStr.WriteString(matches[1])
			inMsgid = false
			inMsgstr = true
			continue
		}

		// Handle multi-line strings (lines that are just quoted values)
		if strings.HasPrefix(line, "\"") && strings.HasSuffix(line, "\"") {
			inner := line[1 : len(line)-1]
			if inMsgid {
				currentKey += inner
			} else if inMsgstr {
				accumulatedStr.WriteString(inner)
			}
		}
	}

	// Flush final pending block
	if currentKey != "" && accumulatedStr.Len() > 0 && locale != "unknown" {
		source.Translations = append(source.Translations, store.TranslationRecord{
			Key:    currentKey,
			Locale: locale,
			Value:  accumulatedStr.String(),
			File:   filePath,
			Format: "po",
		})
	}
}
