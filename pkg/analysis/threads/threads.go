package threads

// Edge defines a directed relationship in the call graph.
type Edge struct {
	From string
	To   string
}

// PropagateThreadColors executes an iterative fixed-point algorithm to diffuse thread identifiers
// from seeding nodes down the execution call graph. Returns a map linking node keys to thread indices.
func PropagateThreadColors(edges []Edge, initialSeeds map[string]int) map[string]int {
	// Initialize map with seed values
	threadAssignments := make(map[string]int)
	for k, v := range initialSeeds {
		threadAssignments[k] = v
	}

	// Iterative propagation
	dirty := true
	for dirty {
		dirty = false
		for _, e := range edges {
			if tid, has := threadAssignments[e.From]; has {
				if _, exists := threadAssignments[e.To]; !exists {
					threadAssignments[e.To] = tid
					dirty = true
				}
			}
		}
	}

	return threadAssignments
}
