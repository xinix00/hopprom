module github.com/xinix00/hopprom

go 1.26.4

require github.com/xinix00/hoplib v0.1.0

require (
	github.com/google/btree v1.1.2 // indirect
	github.com/usbarmory/tamago v1.26.4 // indirect
	github.com/xinix00/go-net v0.1.1-hopos.1 // indirect
	github.com/xinix00/lneto v0.4.0-hopos.1 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	gvisor.dev/gvisor v0.0.0-20250911055229-61a46406f068 // indirect
)

// Alleen voor cmd/hopprom-hopos (tamago-only, zie de build tags daar): het
// HopOS-app-skelet — een echte GitHub-dep (metal/vX.Y.Z-tag in de
// HopOS-repo), dus geen lokale replaces meer nodig; sibling-dev loopt
// via go.work. Host-builds raken deze module dankzij module-pruning
// nooit aan.
require github.com/xinix00/HopOS/metal v1.12.2
