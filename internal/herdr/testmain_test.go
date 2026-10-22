package herdr

import (
	"os"
	"testing"

	"github.com/DnzzL/herdr-docket/internal/testenv"
)

// TestMain keeps this package's tests off the owner's real home, config and
// state dirs (TASK-84); a test that names none of them still gets temp ones.
func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
