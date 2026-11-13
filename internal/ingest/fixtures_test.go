package ingest

import (
	"fmt"
	"os"
	"testing"

	"github.com/Anarchiejs/Vantage/internal/fixtures"
)

func TestMain(m *testing.M) {
	if err := fixtures.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, "fixtures:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
