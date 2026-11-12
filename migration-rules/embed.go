// Package migrationrules embeds the reviewed migration data without a working-directory dependency.
package migrationrules

import _ "embed"

//go:embed 0.31-0.32.json
var data []byte

// JSON returns a private copy of the single, reviewed rules file.
func JSON() []byte { return append([]byte(nil), data...) }
