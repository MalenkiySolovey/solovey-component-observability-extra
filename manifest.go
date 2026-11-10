// Package observabilityextra provides optional history and sampling source.
// Admission and registration belong to the consuming ComponentHost.
package observabilityextra

import (
	_ "embed"
	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
)

//go:embed component.json
var componentJSON []byte

// Manifest returns a fresh descriptor without registering or starting work.
func Manifest() manifest.Manifest { return manifest.MustFromJSON(componentJSON) }
