module github.com/xinix00/hopprom

go 1.26.4

require github.com/xinix00/hoplib v0.1.0

require (
	github.com/usbarmory/tamago v1.26.4 // indirect
	github.com/xinix00/lean v0.99.0 // indirect
)

// Alleen voor cmd/hopprom-hopos (tamago-only, zie de build tags daar): het
// HopOS-app-skelet — een echte GitHub-dep (metal/vX.Y.Z-tag in de
// HopOS-repo), dus geen lokale replaces meer nodig; sibling-dev loopt
// via go.work. Host-builds raken deze module dankzij module-pruning
// nooit aan.
require github.com/xinix00/HopOS/metal v1.99.0
