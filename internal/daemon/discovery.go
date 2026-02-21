package daemon

import (
	"context"
	"io"
	"log"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	"resilient/internal/store"
)

const discoveryNamespace = "resilient-mesh-v1"

// setupDiscovery configures mDNS discovery for local networks
func setupDiscovery(h host.Host, s *store.Store, publish func(string, interface{})) error {
	notifee := &discoveryNotifee{h: h, s: s, publish: publish}
	svc := mdns.NewMdnsService(h, discoveryNamespace, notifee)
	return svc.Start()
}

// setupDHTDiscovery configures Kademlia DHT for wide network finding
func setupDHTDiscovery(ctx context.Context, h host.Host, kDHT *dht.IpfsDHT, s *store.Store, publish func(string, interface{})) error {
	rd := routing.NewRoutingDiscovery(kDHT)
	
	// Expose ourself
	// Note: We don't wait for completion here
	util.Advertise(ctx, rd, discoveryNamespace)

	// Keep looking for peers
	go func() {
		// Use a ticker or simple loop for find peers indefinitely
		// Simplified for now: just poll every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			peers, err := rd.FindPeers(ctx, discoveryNamespace)
			if err != nil {
				log.Printf("DHT FindPeers Error: %v", err)
				<-ticker.C
				continue
			}

			for p := range peers {
				if p.ID == h.ID() {
					continue
				}
				// Connect and store
				if err := h.Connect(ctx, p); err == nil {
					log.Printf("Successfully connected to DHT peer %s", p.ID.String())
					go extractPeerName(ctx, h, p.ID, s)
					var multiaddr string
					if len(p.Addrs) > 0 {
						multiaddr = p.Addrs[0].String()
					}
					peerObj := &store.Peer{
						ID:         p.ID.String(),
						Multiaddr:  multiaddr,
						LastSeen:   time.Now().Unix(),
						TrustLevel: 0,
					}
					s.InsertPeer(peerObj)
					if publish != nil {
						publish("peer_connected", peerObj)
					}
				}
			}
			<-ticker.C
		}
	}()
	
	return nil
}

type discoveryNotifee struct {
	h       host.Host
	s       *store.Store
	publish func(string, interface{})
}

// HandlePeerFound connects to peers discovered via mDNS
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("Discovered new peer via mDNS: %s", pi.ID.String())
	if pi.ID == n.h.ID() {
		return // Ignore self
	}

	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		log.Printf("Error connecting to peer %s: %s", pi.ID.String(), err)
	} else {
		log.Printf("Successfully connected to peer %s", pi.ID.String())
		go extractPeerName(context.Background(), n.h, pi.ID, n.s)
		
		var multiaddrs string
		if len(pi.Addrs) > 0 {
			multiaddrs = pi.Addrs[0].String() // keep simple for now
		}
		
		peerObj := &store.Peer{
			ID:         pi.ID.String(),
			Multiaddr:  multiaddrs,
			LastSeen:   time.Now().Unix(),
			TrustLevel: 0,
		}
		err := n.s.InsertPeer(peerObj)
		if err != nil {
			log.Printf("Failed to insert peer into db: %v", err)
		} else if n.publish != nil {
			n.publish("peer_connected", peerObj)
		}
	}
}

func extractPeerName(ctx context.Context, h host.Host, pid peer.ID, s *store.Store) {
	stream, err := h.NewStream(ctx, pid, identityProtocol)
	if err != nil {
		return
	}
	defer stream.Close()
	stream.SetReadDeadline(time.Now().Add(5 * time.Second))

	buf := make([]byte, 128)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		return
	}

	name := strings.TrimSpace(string(buf[:n]))
	if name != "" {
		s.UpdatePeerName(pid.String(), name)
	}
}
