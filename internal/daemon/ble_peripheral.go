//go:build linux

package daemon

import (
	"context"
	"fmt"
	"log"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

// Custom GATT Service UUID for Resilient Vault
// Randomly generated: 7e4e1704-1e00-4b2e-9d26-000000000001
var serviceUUID, _ = bluetooth.ParseUUID("7e4e1704-1e00-4b2e-9d26-000000000001")

// Characteristic UUID for returning WiFi/Network handoff info
var handoffCharUUID, _ = bluetooth.ParseUUID("7e4e1704-1e00-4b2e-9d26-000000000002")

// SetupBLEBroadcaster initializes the BLE adapter and starts advertising
// a custom GATT service with instructions on how to reach the vault over IP/WiFi.
func SetupBLEBroadcaster(ctx context.Context, apiPort int) error {
	if err := adapter.Enable(); err != nil {
		// Bluetooth may not be available on all devices, we shouldn't fail fatally
		return fmt.Errorf("could not enable BLE adapter (is bluetooth on?): %w", err)
	}

	// Payload data: This provides the Captive Portal or Web Bluetooth PWA client
	// with the credentials needed to join the ad-hoc local network.
	handoffPayload := fmt.Sprintf(`{"ip":"127.0.0.1","port":%d,"ssid":"Resilient-AdHoc","pass":"vaultnet"}`, apiPort)

	// Add the Primary Service
	err := adapter.AddService(&bluetooth.Service{
		UUID: serviceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &bluetooth.Characteristic{},
				UUID:   handoffCharUUID,
				Value:  []byte(handoffPayload),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add BLE service: %w", err)
	}

	// Configure Advertisement Payload
	adv := adapter.DefaultAdvertisement()
	err = adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "ResilientVault",
		ServiceUUIDs: []bluetooth.UUID{serviceUUID},
	})
	if err != nil {
		return fmt.Errorf("failed to configure BLE advertisement: %w", err)
	}

	// Start Broadcasting
	if err := adv.Start(); err != nil {
		return fmt.Errorf("failed to start BLE advertisement: %w", err)
	}

	log.Println("BLE Broadcasting started. Beacon active.")

	// Listen for daemon teardown to gracefully stop broadcasting
	go func() {
		<-ctx.Done()
		log.Println("Stopping BLE Broadcast...")
		adv.Stop()
	}()

	return nil
}
