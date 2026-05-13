package analysis

import (
	"math"

	"doc_generator/pkg/store"
)

// GetCRAPScore calculates Change Risk Anti-Patterns (CRAP) index for a symbol based on complexity and statement coverage.
// C = Complexity. If coverage exists, CRAP = C^2 * (1 - coverage)^3 + C. Otherwise C^2 + C.
func GetCRAPScore(sym store.Symbol) int {
	C := sym.Complexity
	if C <= 0 {
		C = 1
	}
	if sym.Coverage != nil {
		cov := *sym.Coverage / 100.0
		crap := float64(C*C)*math.Pow(1.0-cov, 3) + float64(C)
		return int(math.Round(crap))
	}
	// No coverage counts as 0% covered
	return C*C + C
}
