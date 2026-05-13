package analysis

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestParseJSONTranslations(t *testing.T) {
	var src store.Source
	jsonData := []byte(`{
		"common": {
			"welcome": "Welcome to our app!",
			"nav": {
				"back": "Go Back",
				"next": ""
			}
		},
		"footer": "Copyright 2026"
	}`)

	parseJSONTranslations(&src, "locale/en.json", jsonData, "en")

	expected := map[string]string{
		"common.welcome":  "Welcome to our app!",
		"common.nav.back": "Go Back",
		"footer":          "Copyright 2026",
	}

	if len(src.Translations) != 3 {
		t.Errorf("Expected 3 translations, found %d", len(src.Translations))
	}

	for _, tr := range src.Translations {
		expVal, exists := expected[tr.Key]
		if !exists {
			t.Errorf("Unexpected translation key found: %s", tr.Key)
		} else if tr.Value != expVal {
			t.Errorf("Mismatch for key %s: expected %q, got %q", tr.Key, expVal, tr.Value)
		}
		if tr.Locale != "en" {
			t.Errorf("Expected locale 'en', got %q", tr.Locale)
		}
	}
}

func TestParsePropertiesTranslations(t *testing.T) {
	var src store.Source
	propData := []byte(`# Resource Bundle Header
! System directive
app.title = DocGen Premium Dashboard
app.desc: A static analysis documentation generator.
# Empty/skipped items
app.empty =
`)

	parsePropertiesTranslations(&src, "resources/messages_fr.properties", propData, "fr")

	expected := map[string]string{
		"app.title": "DocGen Premium Dashboard",
		"app.desc":  "A static analysis documentation generator.",
	}

	if len(src.Translations) != 2 {
		t.Errorf("Expected 2 properties translations, found %d", len(src.Translations))
	}

	for _, tr := range src.Translations {
		expVal, exists := expected[tr.Key]
		if !exists {
			t.Errorf("Unexpected key %s", tr.Key)
		} else if tr.Value != expVal {
			t.Errorf("Mismatch for key %s: expected %q, got %q", tr.Key, expVal, tr.Value)
		}
	}
}

func TestParsePOTranslations(t *testing.T) {
	var src store.Source
	poData := []byte(`msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
"Language: ja\n"

# UI Main Screen
msgid "Quit Game"
msgstr "ゲーム終了"

msgid "Long Description"
msgstr ""
"これは複数行に"
"わたる文字列です。"
`)

	parsePOTranslations(&src, "locales/ja.po", poData, "ja")

	expected := map[string]string{
		"Quit Game":        "ゲーム終了",
		"Long Description": "これは複数行にわたる文字列です。",
	}

	if len(src.Translations) != 2 {
		t.Errorf("Expected 2 PO translations, found %d", len(src.Translations))
	}

	for _, tr := range src.Translations {
		expVal, exists := expected[tr.Key]
		if !exists {
			t.Errorf("Unexpected key %s", tr.Key)
		} else if tr.Value != expVal {
			t.Errorf("Mismatch for key %s: expected %q, got %q", tr.Key, expVal, tr.Value)
		}
	}
}
