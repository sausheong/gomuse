// Package web holds the HTML templates and static assets (CSS, JavaScript,
// images) for the Muse web app, embedded into the binary.
package web

import "embed"

// Templates holds the page templates in templates/.
//
//go:embed templates/*.html
var Templates embed.FS

// Static holds the CSS, JavaScript and images served under /static/.
//
//go:embed static
var Static embed.FS
