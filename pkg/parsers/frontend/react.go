package frontend

import (
	"doc_generator/pkg/store"
	"regexp"
	"strings"
)

// IsReactComponent identifies if a function or arrow definition acts as a React Component.
// A basic heuristic is looking for PascalCase names and presence of JSX elements.
func IsReactComponent(name string, bodyText string) bool {
	if len(name) == 0 { return false }
	firstChar := rune(name[0])
	isPascal := firstChar >= 'A' && firstChar <= 'Z'
	
	if !isPascal { return false }

	// Quick check for JSX indicators
	hasJSX := strings.Contains(bodyText, "<") && (strings.Contains(bodyText, "/>") || strings.Contains(bodyText, "</"))
	return hasJSX
}

// RegisterComponent models a detected Javascript component as a uniform Struct/Class symbol in the output.
func RegisterComponent(name, file string, line int, symStore *store.Source) {
	symStore.AddSymbol(store.Symbol{
		Name:    name,
		Kind:    store.SymStruct, // Elevate it to Struct so it shows up in navigation easily
		File:    file,
		Line:    line,
		Doc:     "React Frontend Component",
	})
}

var reJSX = regexp.MustCompile(`<([A-Z][a-zA-Z0-9_]*)(?:\s|/|>)`)

// ExtractJSXCalls scans the code body for custom PascalCase tags indicating nested React components.
// Each finding is registered as a Call relationship mapping parent component to child component.
func ExtractJSXCalls(callerName, bodyText string, symStore *store.Source) {
	matches := reJSX.FindAllStringSubmatch(bodyText, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) >= 2 {
			childName := m[1]
			if !seen[childName] && childName != callerName {
				symStore.AddCall(callerName, childName)
				seen[childName] = true
			}
		}
	}
}
