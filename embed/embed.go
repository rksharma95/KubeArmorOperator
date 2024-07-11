package embed

import (
	"embed"
)

//go:embed *.tgz
var EmbedFs embed.FS
