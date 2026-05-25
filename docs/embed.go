package docs

import "embed"

// CLI embeds the user-facing CLI scenario guides in the Yard binary.
//
//go:embed cli/*.md
var CLI embed.FS
