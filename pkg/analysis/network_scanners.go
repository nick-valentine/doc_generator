package analysis

import (
	"doc_generator/pkg/store"
	"strings"
)

// NetworkScanner defines the strategy interface for specialized heuristic detectors.
type NetworkScanner interface {
	Scan(s store.Symbol, methods []store.Symbol, fields []store.Symbol) *store.NetworkComponent
}

// GameNetcodeScanner isolates mechanics like client-side prediction, lerp, and server rewinding.
type GameNetcodeScanner struct{}

func (g *GameNetcodeScanner) Scan(s store.Symbol, methods []store.Symbol, fields []store.Symbol) *store.NetworkComponent {
	lowerName := strings.ToLower(s.Name)

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
		return &store.NetworkComponent{
			Name:        s.Name,
			Type:        "Latency Mitigation",
			Description: "Detected Client-Side Prediction & Server Reconciliation loop mechanics for authoritative state replay.",
			Symbols:     []string{s.Name},
			Details: map[string]string{
				"Mechanic": "Reconciliation Loop",
				"Safety":   "Authorized History Replay",
			},
		}
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
		return &store.NetworkComponent{
			Name:        s.Name,
			Type:        "Latency Mitigation",
			Description: "Detected Entity Snapshot Interpolation mechanics used to smoothly render remote entity movement from past snapshots.",
			Symbols:     []string{s.Name},
			Details: map[string]string{
				"Mechanic": "Temporal Smoothing (LERP)",
			},
		}
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
		return &store.NetworkComponent{
			Name:        s.Name,
			Type:        "Game Netcode",
			Description: "Implements Server Rewind (Lag Compensation) to temporally shift collision hitboxes for high-latency accuracy.",
			Symbols:     []string{s.Name},
			Details: map[string]string{
				"Approach": "Temporal Rollback / Validation",
			},
		}
	}

	return nil
}

// DistributedScanner identifies cache layers, pubsub architecture, identity validation, and throttling.
type DistributedScanner struct{}

func (d *DistributedScanner) Scan(s store.Symbol, methods []store.Symbol, fields []store.Symbol) *store.NetworkComponent {
	lowerName := strings.ToLower(s.Name)

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
		return &store.NetworkComponent{
			Name:        s.Name,
			Type:        "Realtime Engine",
			Description: "Discovered logical unit orchestrating dynamic subscription propagation, event broadcasting, or presence state.",
			Symbols:     []string{s.Name},
			Details: map[string]string{
				"Pattern":   "Publish/Subscribe",
				"Semantics": "Event Dispatcher",
			},
		}
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
			return &store.NetworkComponent{
				Name:        s.Name,
				Type:        "Caching / Data Hub",
				Description: "Handles intermittent or durable cache storage management, accelerating system read-path throughput.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Backend": "Distributed/Local Cache",
				},
			}
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
			return &store.NetworkComponent{
				Name:        s.Name,
				Type:        "Security Enforcer",
				Description: "Authorizes ingress traffic, validating identities, sessions, or security claims.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Component": "Identity Boundary",
				},
			}
		}
	}

	// 4. Middleware / Traffic Ingress
	isMiddleware := strings.Contains(lowerName, "middleware") || strings.Contains(lowerName, "cors") || strings.Contains(lowerName, "ratelimit") || strings.Contains(lowerName, "throttle")
	if isMiddleware {
		return &store.NetworkComponent{
			Name:        s.Name,
			Type:        "Traffic Control",
			Description: "Regulates or transforms packets in-flight, applying security policies, CORS, or request rate limiters.",
			Symbols:     []string{s.Name},
			Details: map[string]string{
				"Layer": "HTTP/Service Mesh Middleware",
			},
		}
	}

	return nil
}

// TransportScanner isolates TCP, UDP, WebSockets, and API endpoints.
type TransportScanner struct{}

func (t *TransportScanner) Scan(s store.Symbol, methods []store.Symbol, fields []store.Symbol) *store.NetworkComponent {
	lowerName := strings.ToLower(s.Name)

	// Sockets & WebSockets
	if strings.Contains(lowerName, "socket") || strings.Contains(lowerName, "websocket") || strings.Contains(lowerName, "conn") {
		hasUDP := false
		hasTCP := false
		for _, f := range fields {
			lF := strings.ToLower(f.Name + f.Type)
			if strings.Contains(lF, "udp") || strings.Contains(lF, "datagram") || strings.Contains(lF, "kcp") {
				hasUDP = true
			}
			if strings.Contains(lF, "tcp") || strings.Contains(lF, "stream") {
				hasTCP = true
			}
		}

		transport := "Unknown"
		if hasUDP {
			transport = "UDP / Unreliable"
		}
		if hasTCP {
			transport = "TCP / Reliable"
		}
		if strings.Contains(lowerName, "websocket") {
			transport = "WebSocket / Frame-based"
		}

		return &store.NetworkComponent{
			Name:        s.Name,
			Type:        "Transport Layer",
			Description: "Detected persistent communication channel handle facilitating raw data transmission.",
			Symbols:     []string{s.Name},
			Details: map[string]string{
				"Protocol":  transport,
				"Interface": "Persistent Socket Link",
			},
		}
	}

	// HTTP Services & Routers
	if strings.Contains(lowerName, "server") || strings.Contains(lowerName, "router") || strings.Contains(lowerName, "api") || strings.Contains(lowerName, "gateway") {
		isREST := false
		for _, m := range methods {
			lM := strings.ToLower(m.Name)
			if strings.Contains(lM, "get") || strings.Contains(lM, "post") || strings.Contains(lM, "route") || strings.Contains(lM, "handler") {
				isREST = true
			}
		}
		if isREST {
			return &store.NetworkComponent{
				Name:        s.Name,
				Type:        "Edge Service",
				Description: "Detected RESTful or HTTP API service acting as external ingest edge layer.",
				Symbols:     []string{s.Name},
				Details: map[string]string{
					"Protocol": "HTTP/HTTPS",
					"Role":     "Public Endpoints / Controller",
				},
			}
		}
	}

	return nil
}
