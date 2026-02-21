//go:build !linux && !darwin

package daemon

import (
	"context"
	"log"
)

// SetupBLEBroadcaster is a functional stub for operating systems natively lacking
// robust BLE peripheral mode in Go without heavy CGO layers (e.g. macOS).
// On these systems, offline peer discovery will rely predominantly on local mDNS instead.
func SetupBLEBroadcaster(ctx context.Context, apiPort int) error {
	log.Println("Stealth/Local Mode: BLE Broadcasting is not natively supported on this OS (macOS/Windows). Waiting for Wi-Fi mDNS discovery.")
	return nil
}
