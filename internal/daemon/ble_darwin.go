//go:build darwin

package daemon

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
)

var macOSServiceUUID = ble.MustParse("7e4e1704-1e00-4b2e-9d26-000000000001")
var macOSCharUUID = ble.MustParse("7e4e1704-1e00-4b2e-9d26-000000000002")

func SetupBLEBroadcaster(ctx context.Context, apiPort int) error {
	device, err := darwin.NewDevice()
	if err != nil {
		if strings.Contains(err.Error(), "invalid state") || strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			log.Println("\n================================================================")
			log.Println("⚠️  macOS BLUETOOTH PERMISSION REQUIRED ⚠️")
			log.Println("Vault is trying to broadcast its address to offline peers.")
			log.Println("If you did not receive an OS prompt, please grant access manually:")
			log.Println("System Settings -> Privacy & Security -> Bluetooth -> [Your Terminal]")
			log.Println("================================================================\n")
		}
		return fmt.Errorf("failed to initialize macOS BLE device: %w", err)
	}
	ble.SetDefaultDevice(device)

	handoffPayload := fmt.Sprintf(`{"ip":"127.0.0.1","port":%d,"ssid":"Resilient-AdHoc","pass":"vaultnet"}`, apiPort)

	svc := ble.NewService(macOSServiceUUID)
	char := svc.NewCharacteristic(macOSCharUUID)
	char.HandleRead(ble.ReadHandlerFunc(func(req ble.Request, rsp ble.ResponseWriter) {
		rsp.Write([]byte(handoffPayload))
	}))

	if err := ble.AddService(svc); err != nil {
		return fmt.Errorf("failed to add macOS BLE service: %w", err)
	}

	go func() {
		log.Println("BLE Broadcasting started on macOS. Beacon active.")
		if err := ble.AdvertiseNameAndServices(ctx, "ResilientVault", svc.UUID); err != nil {
			log.Printf("macOS BLE advertisement stopped: %v", err)
		}
	}()

	return nil
}
