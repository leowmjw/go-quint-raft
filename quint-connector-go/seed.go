package connector

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

// verbosity is the level of output detail:
//   - 0: minimal (default)
//   - 1: trace and step information
//   - 2: detailed state derivation
var verbosity int

func init() {
	if v, err := strconv.Atoi(os.Getenv("QUINT_VERBOSE")); err == nil {
		verbosity = v
	}
}

// genSeed returns a random seed string for Quint trace generation.
//
// If the QUINT_SEED environment variable is set, its value is used directly.
// Otherwise, a random hex seed is generated for this run.
func genSeed() string {
	if seed := os.Getenv("QUINT_SEED"); seed != "" {
		return seed
	}
	return fmt.Sprintf("0x%x", rand.Uint32()) //nolint:gosec // non-cryptographic seed OK
}

func logTitle(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "== "+format+"\n", args...)
}

// logInfo writes an informational message with indentation.
func logInfo(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "   "+format+"\n", args...)
}

// logSuccess writes a success message with a [OK] prefix.
func logSuccess(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "   [OK] "+format+"\n", args...)
}

// logError writes an error message with a [FAIL] prefix.
func logError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "   [FAIL] "+format+"\n", args...)
}

func logTrace(level int, format string, args ...any) {
	if verbosity >= level {
		fmt.Fprintf(os.Stderr, "   "+format+"\n", args...)
	}
}
