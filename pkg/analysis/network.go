package analysis

import (
	"doc_generator/pkg/store"
	"strings"
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

	// --- Category 1: Game Netcode Logic ---
	for _, s := range allStructs {
		lowerName := strings.ToLower(s.Name)
		methods := structMethods[s.Name]
		fields := structFields[s.Name]

		// Heuristic: Client-Side Prediction & Server Reconciliation
		isRecon := false
		hasTick := false
		for _, m := range methods {
			lowerM := strings.ToLower(m.Name)
			if strings.Contains(lowerM, "reconcile") || strings.Contains(lowerM, "rollback") || strings.Contains(lowerM, "predict") {
				isRecon = true
			}
		}
		for _, f := range fields {
			lowerF := strings.ToLower(f.Name)
			if strings.Contains(lowerF, "tick") || strings.Contains(lowerF, "history") || strings.Contains(lowerF, "buffer") {
				hasTick = true
			}
		}

		if isRecon && hasTick {
			source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
				Name:        s.Name,
				Type:        "Latency Mitigation",
				Description: "Detected Client-Side Prediction & Server Reconciliation loop mechanics for authoritative state replay.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Mechanic": "Reconciliation Loop",
					"Safety":   "Authorized History Replay",
				},
			})
			continue // Avoid doubling up
		}

		// Heuristic: Snapshot Interpolation
		isInterp := false
		hasLerp := false
		if strings.Contains(lowerName, "interpolate") || strings.Contains(lowerName, "snapshot") {
			isInterp = true
		}
		for _, m := range methods {
			lowerM := strings.ToLower(m.Name)
			if strings.Contains(lowerM, "lerp") || strings.Contains(lowerM, "slerp") || strings.Contains(lowerM, "interpolate") {
				hasLerp = true
			}
		}
		if isInterp || (hasLerp && hasTick) {
			source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
				Name:        s.Name,
				Type:        "Latency Mitigation",
				Description: "Detected Entity Snapshot Interpolation mechanics used to smoothly render remote entity movement from past snapshots.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Mechanic": "Temporal Smoothing (LERP)",
				},
			})
			continue
		}
		
		// Heuristic: Lag Compensation / Server Rewind
		isCompensate := false
		if strings.Contains(lowerName, "lag") || strings.Contains(lowerName, "compensat") || strings.Contains(lowerName, "rewind") {
			isCompensate = true
		}
		for _, m := range methods {
			lowerM := strings.ToLower(m.Name)
			if strings.Contains(lowerM, "validate") || strings.Contains(lowerM, "hitscan") || strings.Contains(lowerM, "rewind") {
				if isCompensate || strings.Contains(lowerM, "compensat") {
					isCompensate = true
				}
			}
		}
		if isCompensate {
			source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
				Name:        s.Name,
				Type:        "Game Netcode",
				Description: "Implements Server Rewind (Lag Compensation) to temporally shift collision hitboxes for high-latency accuracy.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Approach": "Temporal Rollback / Validation",
				},
			})
		}
	}

	// --- Category 2: Web & Distributed System Components ---
	for _, s := range allStructs {
		lowerName := strings.ToLower(s.Name)
		methods := structMethods[s.Name]

		// 1. Realtime Messaging / PubSub Architecture
		isPubSub := false
		if strings.Contains(lowerName, "pubsub") || strings.Contains(lowerName, "broadcast") || strings.Contains(lowerName, "hub") || strings.Contains(lowerName, "channel") {
			isPubSub = true
		}
		for _, m := range methods {
			lm := strings.ToLower(m.Name)
			if strings.Contains(lm, "subscribe") || strings.Contains(lm, "publish") || strings.Contains(lm, "presence") || strings.Contains(lm, "notify") {
				isPubSub = true
			}
		}
		if isPubSub && !strings.Contains(lowerName, "io.") {
			source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
				Name:        s.Name,
				Type:        "Realtime Engine",
				Description: "Discovered logical unit orchestrating dynamic subscription propagation, event broadcasting, or presence state.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Pattern": "Publish/Subscribe",
					"Semantics": "Event Dispatcher",
				},
			})
		}

		// 2. Distributed Cache Layer
		isCache := strings.Contains(lowerName, "cache") || strings.Contains(lowerName, "redis") || strings.Contains(lowerName, "memcached")
		if isCache {
			hasHits := false
			for _, m := range methods {
				lm := strings.ToLower(m.Name)
				if strings.Contains(lm, "get") || strings.Contains(lm, "set") || strings.Contains(lm, "flush") || strings.Contains(lm, "invalidate") {
					hasHits = true
				}
			}
			if hasHits {
				source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
					Name:        s.Name,
					Type:        "Caching / Data Hub",
					Description: "Handles intermittent or durable cache storage management, accelerating system read-path throughput.",
					Symbols:     []string{s.Name},
					Details: map[string]string{
						"Backend": "Distributed/Local Cache",
					},
				})
			}
		}

		// 3. Authentication / Identity Access
		isAuth := strings.Contains(lowerName, "auth") || strings.Contains(lowerName, "oauth") || strings.Contains(lowerName, "token") || strings.Contains(lowerName, "session")
		if isAuth {
			hasValid := false
			for _, m := range methods {
				lm := strings.ToLower(m.Name)
				if strings.Contains(lm, "valid") || strings.Contains(lm, "check") || strings.Contains(lm, "login") || strings.Contains(lm, "verif") {
					hasValid = true
				}
			}
			if hasValid {
				source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
					Name:        s.Name,
					Type:        "Security Enforcer",
					Description: "Authorizes ingress traffic, validating identities, sessions, or security claims.",
					Symbols:     []string{s.Name},
					Details: map[string]string{
						"Component": "Identity Boundary",
					},
				})
			}
		}

		// 4. Middleware / Traffic Ingress
		isMiddleware := strings.Contains(lowerName, "middleware") || strings.Contains(lowerName, "cors") || strings.Contains(lowerName, "ratelimit") || strings.Contains(lowerName, "throttle")
		if isMiddleware {
			source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
				Name:        s.Name,
				Type:        "Traffic Control",
				Description: "Regulates or transforms packets in-flight, applying security policies, CORS, or request rate limiters.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Layer": "HTTP/Service Mesh Middleware",
				},
			})
		}
	}

	// --- Category 3: Services & Transport Protocols ---
	for _, s := range allStructs {
		lowerName := strings.ToLower(s.Name)
		
		// Sockets & WebSockets
		if strings.Contains(lowerName, "socket") || strings.Contains(lowerName, "websocket") || strings.Contains(lowerName, "conn") {
			// Verify connection type via field analysis
			hasUDP := false
			hasTCP := false
			for _, f := range structFields[s.Name] {
				lF := strings.ToLower(f.Name + f.Type)
				if strings.Contains(lF, "udp") || strings.Contains(lF, "datagram") || strings.Contains(lF, "kcp") {
					hasUDP = true
				}
				if strings.Contains(lF, "tcp") || strings.Contains(lF, "stream") {
					hasTCP = true
				}
			}
			
			transport := "Unknown"
			if hasUDP { transport = "UDP / Unreliable" }
			if hasTCP { transport = "TCP / Reliable" }
			if strings.Contains(lowerName, "websocket") { transport = "WebSocket / Frame-based" }

			source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
				Name:        s.Name,
				Type:        "Transport Layer",
				Description: "Detected persistent communication channel handle facilitating raw data transmission.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Protocol":  transport,
					"Interface": "Persistent Socket Link",
				},
			})
		}

		// HTTP Services & Routers
		if strings.Contains(lowerName, "server") || strings.Contains(lowerName, "router") || strings.Contains(lowerName, "api") || strings.Contains(lowerName, "gateway") {
			// Distinct API layer
			isREST := false
			methods := structMethods[s.Name]
			for _, m := range methods {
				lM := strings.ToLower(m.Name)
				if strings.Contains(lM, "get") || strings.Contains(lM, "post") || strings.Contains(lM, "route") || strings.Contains(lM, "handler") {
					isREST = true
				}
			}
			if isREST {
				source.NetworkAnalysis = append(source.NetworkAnalysis, store.NetworkComponent{
					Name:        s.Name,
					Type:        "Edge Service",
					Description: "Detected RESTful or HTTP API service acting as external ingest edge layer.",
					Symbols:     []string{s.Name},
					Details: map[string]string{
						"Protocol": "HTTP/HTTPS",
						"Role":     "Public Endpoints / Controller",
					},
				})
			}
		}
	}

	// --- Category 4: Global Callgraph Context & Risk Profiling ---
	// We perform a final post-process to assess connection weights and blast radius.
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
