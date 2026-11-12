package ospath

import (
	"runtime"
	"testing"
)

func TestForCompareLeavesOtherSystemsAlone(t *testing.T) {
	samples := []string{"", "plain.md", `C:\Fixtures\Compass`, "C:/Fixtures/Test", "Na\u00efve.md", "/tmp/vault"}
	for _, sample := range samples {
		if got := ForCompare(sample); got != sample {
			t.Fatalf("ForCompare(%q) = %q", sample, got)
		}
	}
	nfd := "Nai\u0308ve.md"
	got := ForCompare(nfd)
	if runtime.GOOS == "darwin" {
		if got != "Na\u00efve.md" {
			t.Fatalf("ForCompare(%q) = %q", nfd, got)
		}
		return
	}
	if got != nfd {
		t.Fatalf("ForCompare rewrote %q to %q", nfd, got)
	}
}
