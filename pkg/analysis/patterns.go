package analysis

import (
	"doc_generator/pkg/store"
	"strings"
)

// RunPatternAnalysis executes comprehensive heuristic passes over the current source cache,
// seeking common creational, structural, behavioral, and game design patterns.
func RunPatternAnalysis(source *store.Source) {
	// Map of struct fields and methods for structural and relational trace
	structFields := make(map[string][]store.Symbol)
	methodsByParent := make(map[string][]store.Symbol)
	
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymField {
			structFields[sym.Parent] = append(structFields[sym.Parent], sym)
		} else if sym.Kind == store.SymMethod {
			methodsByParent[sym.Parent] = append(methodsByParent[sym.Parent], sym)
		}
	}

	allStructs := getSymbols(source, store.SymStruct)
	allInterfaces := getSymbols(source, store.SymInterface)

	// --- 1. Verified Singleton Detection ---
	// Stricter Heuristic: Global variable holding specific anchoring names OR private holders accessed by distinct synced accessor.
	for _, v := range getSymbols(source, store.SymVariable) {
		vType := strings.TrimPrefix(v.Type, "*")
		lowerV := strings.ToLower(v.Name)
		// Explicit anchoring marker keywords
		isAnchored := strings.Contains(lowerV, "instance") || strings.Contains(lowerV, "default") || lowerV == "singleton" || lowerV == "current" || lowerV == "global"
		
		if isAnchored {
			for _, s := range allStructs {
				if v.Package == s.Package && (vType == s.Name || strings.HasSuffix(vType, "."+s.Name)) {
					source.AddPattern(store.Pattern{
						Name:        "Singleton",
						Description: "Explicitly anchored global instance providing centralized controlled state.",
						Category:    "Creational",
						Symbols:     []string{s.Name, v.Name},
					})
					break
				}
			}
		}
	}

	// --- 2. Behavioral Factory Method Detection ---
	// Heuristic: Struct methods return other concrete types OR generalized interfaces DIFFERENT from self.
	for _, s := range allStructs {
		createCount := 0
		var createdTypes []string
		
		for _, m := range methodsByParent[s.Name] {
			lowerM := strings.ToLower(m.Name)
			mType := strings.TrimPrefix(m.Type, "*")
			
			// If method is named creationary AND signature return differs from own struct
			isCreationary := strings.HasPrefix(lowerM, "create") || strings.HasPrefix(lowerM, "new") || strings.HasPrefix(lowerM, "build") || strings.HasPrefix(lowerM, "spawn")
			
			// Try basic validation if return contains reference to other known structs
			differsFromSelf := !strings.Contains(mType, s.Name)
			if isCreationary && differsFromSelf && mType != "" && mType != "void" {
				createCount++
				createdTypes = append(createdTypes, mType)
			}
		}

		// If it explicitly creates multiple external artifacts OR holds "Factory" suffix + creation behavior
		isFactory := (createCount >= 2) || (strings.HasSuffix(strings.ToLower(s.Name), "factory") && createCount >= 1)
		if isFactory {
			source.AddPattern(store.Pattern{
				Name:        "Factory Method / Abstract Factory",
				Description: "Identified struct performing intentional object manufacture, yielding objects derived outside its own receiver scope.",
				Category:    "Creational",
				Symbols:     append([]string{s.Name}),
			})
		}
	}

	// --- 3. Confirmed Composite / Self-Recursive Structs ---
	// Heuristic: A struct that physically contains a field or slice of its own type. Pure structural indicator.
	for _, s := range allStructs {
		fields := structFields[s.Name]
		isRecursive := false
		for _, f := range fields {
			fType := strings.Trim(strings.TrimPrefix(f.Type, "*"), "[] ")
			if fType == s.Name || strings.HasSuffix(fType, "."+s.Name) {
				isRecursive = true
				break
			}
		}
		if isRecursive {
			source.AddPattern(store.Pattern{
				Name:        "Composite / Chain of Responsibility",
				Description: "Recursive composition hierarchy detected where structure encapsulates recursive instances of itself.",
				Category:    "Structural",
				Symbols:     []string{s.Name},
			})
		}
	}

	// --- 4. Strategy / Bridge Delegation ---
	for _, s := range allStructs {
		fields := structFields[s.Name]
		var injectedInterfaces []string
		for _, f := range fields {
			fType := strings.Trim(strings.TrimPrefix(f.Type, "*"), "[] ")
			// Cross-reference against absolute known interfaces in catalog
			for _, i := range allInterfaces {
				if i.Name == fType || strings.HasSuffix(fType, "."+i.Name) {
					injectedInterfaces = append(injectedInterfaces, i.Name)
				}
			}
		}

		if len(injectedInterfaces) > 0 {
			// Check if this struct implements the same injected interface (Structural Decorator)
			isDecorator := false
			for _, ri := range injectedInterfaces {
				for _, rel := range s.Relations {
					if strings.Contains(rel, ri) {
						isDecorator = true
						break
					}
				}
			}

			if isDecorator {
				source.AddPattern(store.Pattern{
					Name:        "Decorator / Wrapper",
					Description: "Implements identical interface to an embedded component, wrapping behavioral routing.",
					Category:    "Structural",
					Symbols:     append([]string{s.Name}, injectedInterfaces...),
				})
			} else {
				// Strategy: Injects variable behavior via externalized interface boundary
				source.AddPattern(store.Pattern{
					Name:        "Strategy / Adapter",
					Description: "Encapsulates runtime algorithm family by holding reference to externalized abstract interface.",
					Category:    "Behavioral",
					Symbols:     append([]string{s.Name}, injectedInterfaces...),
				})
			}
		}
	}

	// --- 5. Confirmed Container Observer ---
	// Heuristic: Struct holds collections of callback pointers/interfaces AND exhibits distribution methods.
	for _, s := range allStructs {
		hasCollection := false
		fields := structFields[s.Name]
		for _, f := range fields {
			// Contains slice, map, or explicitly named registry holding functions or handler objects
			lowF := strings.ToLower(f.Type)
			if strings.HasPrefix(f.Type, "[]") || strings.Contains(lowF, "map") || strings.Contains(strings.ToLower(f.Name), "listener") {
				hasCollection = true
				break
			}
		}

		if hasCollection {
			hasRegister := false
			hasEmit := false
			for _, m := range methodsByParent[s.Name] {
				lowM := strings.ToLower(m.Name)
				if strings.Contains(lowM, "subscribe") || strings.Contains(lowM, "register") || strings.Contains(lowM, "add") {
					hasRegister = true
				}
				if strings.Contains(lowM, "notify") || strings.Contains(lowM, "publish") || strings.Contains(lowM, "emit") || strings.Contains(lowM, "broadcast") {
					hasEmit = true
				}
			}

			if hasRegister && hasEmit {
				source.AddPattern(store.Pattern{
					Name:        "Observer / PubSub",
					Description: "One-to-many registry architecture tracking list of dependents and distributing state events natively.",
					Category:    "Behavioral",
					Symbols:     []string{s.Name},
				})
			}
		}
	}

	// --- 6. Object Pool Management ---
	// Heuristic: Manages collection, AND contains explicit Checkout/Return logic semantics.
	for _, s := range allStructs {
		if strings.HasSuffix(strings.ToLower(s.Name), "pool") {
			hasAlloc := false
			for _, m := range methodsByParent[s.Name] {
				lowM := strings.ToLower(m.Name)
				if strings.Contains(lowM, "get") || strings.Contains(lowM, "put") || strings.Contains(lowM, "acquire") || strings.Contains(lowM, "release") {
					hasAlloc = true
				}
			}
			if hasAlloc {
				source.AddPattern(store.Pattern{
					Name:        "Object Pool",
					Description: "Resource conservation unit managing allocation, retrieval, and disposal cycles.",
					Category:    "Optimization",
					Symbols:     []string{s.Name},
				})
			}
		}
	}
}

func getSymbols(source *store.Source, kind store.SymbolType) []store.Symbol {
	var res []store.Symbol
	for _, s := range source.Symbols {
		if s.Kind == kind {
			res = append(res, s)
		}
	}
	return res
}
