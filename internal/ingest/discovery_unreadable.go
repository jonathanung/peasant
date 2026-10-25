package ingest

import (
	"errors"
	"fmt"
	"io/fs"
)

// unreadableSourceDiagnostic turns a permission error met while walking a
// source tree into a discovery diagnostic. Other walk errors (a file removed
// mid-walk, a missing root) stay silent as before.
func unreadableSourceDiagnostic(harness Harness, path string, err error) (DiscoveryDiagnostic, bool) {
	if !errors.Is(err, fs.ErrPermission) {
		return DiscoveryDiagnostic{}, false
	}
	return DiscoveryDiagnostic{
		Provider: harness,
		Code:     "source_unreadable",
		Location: path,
		Summary:  "unreadable source directory",
		Detail:   fmt.Sprintf("%v; sessions under it were not read; fix its permissions and rerun harvest", err),
	}, true
}
