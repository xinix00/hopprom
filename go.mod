module hopprom

go 1.26.4

require github.com/xinix00/hoplib v0.1.0

require (
	github.com/google/btree v1.1.2 // indirect
	github.com/soypat/lneto v0.1.1-0.20260609173350-82f946154800 // indirect
	github.com/usbarmory/go-net v0.0.0-20260626130943-dad9ef39fd9b // indirect
	github.com/usbarmory/tamago v1.26.4 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	gvisor.dev/gvisor v0.0.0-20250911055229-61a46406f068 // indirect
)

// Alleen voor cmd/hopprom-hopos (tamago-only, zie de build tags daar): het
// HopOS-app-skelet. Replaces gelden niet transitief, dus hop-os/metal's eigen
// `replace hop => …` staat hier herhaald. Lokale paden — net als hop-os-surf
// bouwt dit alleen op een werkplek met de sibling-checkouts; de host-builds
// (cmd/hopprom, ./...) raken deze modules dankzij module-pruning nooit aan.
require hop-os/metal v1.5.5

replace (
	hop => ../hop
	hop-os/metal => ../../hop-os/metal
)
