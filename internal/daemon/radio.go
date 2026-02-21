package daemon

import (
	"context"
	"io"
	"log"

	"go.bug.st/serial"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

// HardwareBridge interface defines a way to send and receive raw bytes over a physical radio
type HardwareBridge interface {
	io.ReadWriteCloser
}

// LoRaBridge connects a serial connection to a LoRa Radio Transceiver (e.g. Meshtastic node)
type LoRaBridge struct {
	port serial.Port
}

func NewLoRaBridge(portName string, baud int) (*LoRaBridge, error) {
	mode := &serial.Mode{
		BaudRate: baud,
	}
	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}
	return &LoRaBridge{port: port}, nil
}

func (b *LoRaBridge) Read(p []byte) (n int, err error) {
	return b.port.Read(p)
}

func (b *LoRaBridge) Write(p []byte) (n int, err error) {
	return b.port.Write(p)
}

func (b *LoRaBridge) Close() error {
	return b.port.Close()
}

// SetupHardwareBridges configures low bandwidth packet-radio bridges
// and mounts them onto the libp2p network host as a custom transport
func SetupHardwareBridges(ctx context.Context, h host.Host, portName string, baud int) error {
	log.Printf("Initializing LoRa/BLE Hardware Bridge on %s at %d baud", portName, baud)
	
	bridge, err := NewLoRaBridge(portName, baud)
	if err != nil {
		log.Printf("Failed to bind serial radio: %v. Continuing without hardware transport.", err)
		return nil // don't fail the whole daemon if the radio is unplugged
	}

	// Since libp2p transports require heavy multiplexing (mplex/yamux), 
	// running them over 900mhz LoRa directly is very difficult due to MTU size (256 bytes mostly).
	// We establish a custom raw stream handler that can accept tunneled requests, bypassing standard multiplexers 
	// for strictly text-based mesh communication when standard TCP/QUIC transports are offline.
	h.SetStreamHandler("/resilient/radio/1.0.0", func(s network.Stream) {
		defer s.Close()
		log.Printf("Received inbound stream from radio peer: %s", s.Conn().RemotePeer())
		
		// Bridge bytes from the stream to the physical radio
		io.Copy(bridge, s)
	})

	return nil
}
