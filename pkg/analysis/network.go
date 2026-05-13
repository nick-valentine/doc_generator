package analysis

import (
	"doc_generator/pkg/store"
	"fmt"
)

// RunNetworkAnalysis executes specialized scans across the symbol table to pinpoint networking architecture,
// service layers, and game netcode components (e.g. reconciliation, interpolation).
func RunNetworkAnalysis(source *store.Source) {
	// Preparation maps for speed
	structMethods := make(map[string][]store.Symbol)
	structFields := make(map[string][]store.Symbol)
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymMethod {
			structMethods[sym.Parent] = append(structMethods[sym.Parent], sym)
		} else if sym.Kind == store.SymField {
			structFields[sym.Parent] = append(structFields[sym.Parent], sym)
		}
	}

	allStructs := getSymbols(source, store.SymStruct)

	// Register standard network scanners
	scanners := []NetworkScanner{
		&GameNetcodeScanner{},
		&DistributedScanner{},
		&TransportScanner{},
	}

	// Execute scanning strategy pattern across structs
	for _, s := range allStructs {
		methods := structMethods[s.Name]
		fields := structFields[s.Name]

		for _, scanner := range scanners {
			if comp := scanner.Scan(s, methods, fields); comp != nil {
				source.NetworkAnalysis = append(source.NetworkAnalysis, *comp)
				break // Stop scanning once classified to prevent duplicate components
			}
		}
	}

	// Execute final risk context profiling pass
	runRiskProfiling(source, structMethods)
}

// runRiskProfiling performs a final post-process over detected network nodes to assess connection weights and blast radius.
func runRiskProfiling(source *store.Source, structMethods map[string][]store.Symbol) {
	for i := range source.NetworkAnalysis {
		comp := &source.NetworkAnalysis[i]
		if comp.Details == nil {
			comp.Details = make(map[string]string)
		}

		totalInbound := 0
		totalOutbound := 0
		dependents := make(map[string]bool)
		fanout := make(map[string]bool)

		// Gather aggregate call volume for all symbols bound to this component
		for _, symName := range comp.Symbols {
			callers := source.GetCallers(symName)
			for _, c := range callers {
				dependents[c] = true
			}
			callees := source.GetCallees(symName)
			for _, ce := range callees {
				fanout[ce] = true
			}

			// Also check methods to get richer connectivity for structs
			methods := structMethods[symName]
			for _, m := range methods {
				fullName := symName + "." + m.Name
				for _, c := range source.GetCallers(fullName) {
					dependents[c] = true
				}
				for _, ce := range source.GetCallees(fullName) {
					fanout[ce] = true
				}
			}
		}
		totalInbound = len(dependents)
		totalOutbound = len(fanout)

		// Assign Connectivity Metadata
		comp.Details["Blast Radius"] = fmt.Sprintf("%d downstream systems", totalOutbound)
		comp.Details["System Weight"] = fmt.Sprintf("%d distinct callers", totalInbound)

		// Derive Risk Score based on profile type and connectivity
		risk := "Low"
		riskColor := "🟢"
		if comp.Type == "Security Enforcer" {
			if totalInbound > 15 {
				risk = "Critical Central Chokepoint"
				riskColor = "🔴"
			} else if totalInbound > 5 {
				risk = "High"
				riskColor = "🟠"
			}
		} else if comp.Type == "Edge Service" {
			if totalInbound > 20 {
				risk = "High Traffic Entrypoint"
				riskColor = "🟠"
			}
		} else if comp.Type == "Realtime Engine" {
			if totalOutbound > 20 {
				risk = "High Broadcaster Load"
				riskColor = "🟠"
			}
		}

		// High coupling risk
		if totalOutbound > 30 {
			risk = "Monolithic Dependency Lock"
			riskColor = "🔴"
		}

		comp.Details["Security & Risk Context"] = fmt.Sprintf("%s %s", riskColor, risk)
		
		// Value Add determination
		if totalInbound == 0 && totalOutbound > 0 {
			comp.Details["Architecture Role"] = "Root Orchestrator / Daemon"
		} else if totalInbound > 10 && totalOutbound < 5 {
			comp.Details["Architecture Role"] = "Highly Reusable Core Utility"
		}
	}
}
