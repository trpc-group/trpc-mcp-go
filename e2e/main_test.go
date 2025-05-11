package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
)

// Global flags.
var (
	// useRealServer controls whether to use a real HTTP server.
	useRealServer = flag.Bool("real", false, "Use real HTTP server instead of httptest.")

	// serverAddr specifies the real server address.
	serverAddr = flag.String("addr", "localhost:3456", "Real server address.")

	// verboseLogging controls verbose logging.
	verboseLogging = flag.Bool("verbose", false, "Enable verbose logging.")

	// TestGetSSEEnabled controls whether to enable GET SSE tests.
	TestGetSSEEnabled = flag.Bool("getsse", true, "Enable GET SSE tests.")
)

// Global timeout settings.
const (
	// Default test timeout.
	defaultTestTimeout = 5 * time.Second

	// Long test timeout.
	longTestTimeout = 10 * time.Second
)

// TestMain sets up the environment for the entire test package.
func TestMain(m *testing.M) {
	// Parse command line flags.
	flag.Parse()

	// Set log prefix.
	if *verboseLogging {
		fmt.Println("Verbose logging mode enabled.")
	}

	// Run tests.
	exitCode := m.Run()

	// Exit.
	os.Exit(exitCode)
}

// Helper function: get full test name based on short name.
func getTestName(t *testing.T, shortName string) string {
	return fmt.Sprintf("%s/%s", t.Name(), shortName)
}
