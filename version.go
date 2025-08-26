package root

import (
	_ "embed"
)

//go:embed build_version.json
var BuildVersion string
