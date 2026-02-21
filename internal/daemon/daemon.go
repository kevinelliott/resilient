package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"

	"resilient/internal/api"
	"resilient/internal/cas"
	"resilient/internal/store"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/x/rate"
)

const identityProtocol = "/resilient/identity/1.0.0"

type Config struct {
	DBPath  string
	CASDir  string
	APIPort int
	P2PPort int
	Profile string // "hub", "stealth", "standard"
}

type Daemon struct {
	ctx    context.Context
	cfg    *Config
	host   host.Host
	dht      *dht.IpfsDHT
	pubsub   *pubsub.PubSub
	chatRoom *ChatRoom
	catalog  *CatalogAnnouncer // Added this field
	casSync  *CASSyncManager   // Added this field
	store    *store.Store
	cas      *cas.Store
	api      *api.Server
}

func New(ctx context.Context, cfg *Config) (*Daemon, error) {
	// Initialize the database store
	s, err := store.New(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init store: %w", err)
	}

	// Initialize CAS store
	cStore, err := cas.New(cfg.CASDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init cas: %w", err)
	}

	// Initialize Infinite Resource Manager to prevent "rate limit exceeded" on mass loopback swarm testing
	limiter := rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits)
	rm, err := rcmgr.NewResourceManager(
		limiter,
		rcmgr.WithConnRateLimiters(&rate.Limiter{
			GlobalLimit: rate.Limit{}, // empty limits bypass everything
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rcmgr: %w", err)
	}
    
	// Override default Connmgr so it doesn't aggressively cull our 50 test nodes
	cm, err := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create connmgr: %w", err)
	}

	// Initialize libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.P2PPort),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", cfg.P2PPort),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/webrtc-direct", cfg.P2PPort),
		),
		libp2p.ResourceManager(rm),
		libp2p.ConnectionManager(cm),
		libp2p.DisableRelay(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Initialize Kademlia DHT
	kDHT, err := dht.New(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	// Initialize GossipSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Setup ChatRoom
	cr, err := setupChatRoom(ctx, h, ps, s)
	if err != nil {
		return nil, fmt.Errorf("failed to setup chat room: %w", err)
	}

	// Setup Catalog Syncing
	ca, err := setupCatalogAnnouncer(ctx, h, ps, s)
	if err != nil {
		return nil, fmt.Errorf("failed to setup catalog announcer: %w", err)
	}

	// Setup CAS Syncing
	casSync := setupCASSyncManager(ctx, h, cStore)

	// Seed Mock Catalog Data for UI Demo (only if empty)
	cats, _ := s.GetCatalogs()
	if len(cats) == 0 {
		s.InsertCatalog(&store.Catalog{ID: "cat1", Name: "Wikipedia EN (ZIM)", Description: "Full offline wikipedia", RootHash: "root123", CreatedAt: 1718000000})
		s.InsertCatalog(&store.Catalog{ID: "cat2", Name: "Survival Manuals Vol 1", Description: "PDFs for offgrid living", RootHash: "root456", CreatedAt: 1718000010})
		s.InsertFile(&store.File{ID: "f1", CatalogID: "cat1", Path: "wiki_en.zim", Size: 92400000000, ChunkHashes: `["hash1", "hash2"]`})
		s.InsertFile(&store.File{ID: "f2", CatalogID: "cat2", Path: "water_purification.pdf", Size: 1200000, ChunkHashes: `["hash3"]`})
	}

	// Initialize the API server
	apiSrv := api.New(cfg.APIPort, h, time.Now(), s, cStore, cr, casSync, func() error {
		return BootstrapDefaultData(s)
	})

	return &Daemon{
		ctx:      ctx,
		cfg:      cfg,
		host:     h,
		dht:      kDHT,
		pubsub:   ps,
		chatRoom: cr,
		catalog:  ca,
		casSync:  casSync,
		store:    s,
		cas:      cStore,
		api:      apiSrv,
	}, nil
}

func (d *Daemon) Start() error {
	log.Printf("Daemon started. Peer ID: %s", d.host.ID())
	log.Printf("Listening on: %v", d.host.Addrs())

	var conf *store.NodeConfig
	if c, err := d.store.GetConfig(); err == nil {
		conf = c
		d.cfg.Profile = conf.NodeProfile
	} else {
		log.Printf("Failed to load node config, proceeding with defaults: %v", err)
	}

	// Bootstrap default survival catalogs if DB is brand new
	if err := BootstrapDefaultData(d.store); err != nil {
		log.Printf("Failed to inject default survival catalog: %v", err)
	}
	
	// Start HTTP API
	go func() {
		if err := d.api.Start(); err != nil {
			log.Printf("API server stopped: %v", err)
		}
	}()

	// Setup Identity Protocol
	d.host.SetStreamHandler(identityProtocol, func(stream network.Stream) {
		defer stream.Close()
		if d.cfg.Profile == "stealth" || conf == nil || conf.NodeName == "" {
			return
		}
		stream.Write([]byte(conf.NodeName))
	})

	// Bootstrap DHT
	if err := d.dht.Bootstrap(d.ctx); err != nil {
		log.Printf("Failed to bootstrap DHT: %v", err)
	} else {
		// Only start DHT discovery if bootstrap succeeds or we don't care about failure
		if err := setupDHTDiscovery(d.ctx, d.host, d.dht, d.store, d.api.PublishEvent); err != nil {
			log.Printf("Failed to setup DHT discovery: %v", err)
		} else {
			log.Println("DHT discovery started.")
		}
	}

	// Start Discovery (mDNS)
	if d.cfg.Profile != "stealth" {
		if err := setupDiscovery(d.host, d.store, d.api.PublishEvent); err != nil {
			log.Printf("Failed to setup mDNS discovery: %v", err)
		} else {
			log.Println("mDNS discovery started.")
		}
	} else {
		log.Println("Stealth Mode: mDNS active discovery disabled.")
	}

	// Setup Experimental Radio Bridge
	loraPort := "/dev/ttyUSB0"
	loraBaud := 115200
	if conf != nil && conf.LoraPort != "" {
		loraPort = conf.LoraPort
		loraBaud = conf.LoraBaud
	}
	if err := SetupHardwareBridges(d.ctx, d.host, loraPort, loraBaud); err != nil {
		log.Printf("Failed to setup hardware radio bridge: %v", err)
	}

	// Setup BLE Discovery Beacon (Captive Portal Handoff)
	if d.cfg.Profile != "stealth" {
		bleEnabled := true
		if conf != nil {
			bleEnabled = conf.BleEnabled
		}
		if bleEnabled {
			if err := SetupBLEBroadcaster(d.ctx, d.cfg.APIPort); err != nil {
				log.Printf("Failed to setup BLE broadcast beacon: %v", err)
			}
		} else {
			log.Println("BLE broadcasting disabled by configuration.")
		}
	} else {
		log.Println("Stealth Mode: BLE active broadcasting disabled.")
	}

	// Periodically announce catalogs
	if d.cfg.Profile != "stealth" {
		go func() {
			// Announce immediately
			d.catalog.AnnounceAll()

			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-d.ctx.Done():
					return
				case <-ticker.C:
					d.catalog.AnnounceAll()
				}
			}
		}()
	} else {
		log.Println("Stealth Mode: Catalog periodic public announcements disabled.")
	}

	return nil
}

func (d *Daemon) Stop() error {
	log.Println("Stopping daemon...")
	if err := d.api.Stop(context.Background()); err != nil {
		log.Printf("Error stopping API: %v", err)
	}
	if err := d.dht.Close(); err != nil {
		log.Printf("Error closing DHT: %v", err)
	}
	if err := d.host.Close(); err != nil {
		return err
	}
	if err := d.store.Close(); err != nil {
		return err
	}
	return nil
}
