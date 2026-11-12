// Package ospath adjusts filesystem paths for the host operating system.
// On Linux and Windows every function returns its input unchanged.
// On macOS, APFS stores filenames in Unicode NFD. NFC is what Linux, Windows,
// and this repository use, and what path comparisons need in order to match.
package ospath

import (
	"runtime"

	"golang.org/x/text/unicode/norm"
)

// ForCompare returns path suitable for comparison and for storage.
// The result is identical to path on Linux and Windows, including when path
// is already decomposed.
func ForCompare(path string) string {
	if runtime.GOOS != "darwin" || path == "" || norm.NFC.IsNormalString(path) {
		return path
	}
	return norm.NFC.String(path)
}
