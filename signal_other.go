//go:build !unix

package main

import "os"

// upgradeSignal returns nil on platforms without a live-upgrade signal, so no
// upgrade handler is registered.
func upgradeSignal() os.Signal { return nil }
