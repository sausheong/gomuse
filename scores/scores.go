// Package scores embeds the sample scores so the web app can serve them
// from /sample/{name} without needing the files on disk.
package scores

import "embed"

// FS holds every *.yaml sample score in this directory.
//
//go:embed *.yaml
var FS embed.FS
