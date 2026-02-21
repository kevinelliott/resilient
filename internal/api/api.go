package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
	"resilient/internal/store"
	"resilient/internal/cas"
)

type Publisher interface {
	Publish(content string, refTargetID string) (*store.SocialMessage, error)
	PublishDirect(content string, recipientID string) (*store.SocialMessage, error)
}

type Fetcher interface {
	FetchFile(ctx context.Context, peerID peer.ID, file *store.File, publish func(string, interface{})) error
	FetchFileSwarm(ctx context.Context, peers []peer.ID, file *store.File, publish func(string, interface{})) error
	GetActiveDownloads() map[string]interface{}
}

type Server struct {
	port             int
	nodeID           string
	startTime        time.Time
	srv              *http.Server
	host             host.Host
	store            *store.Store
	cas              *cas.Store
	publisher        Publisher
	fetcher          Fetcher
	eventBus         *EventBus

	ingestMu         sync.RWMutex
	activeIngestions map[string]map[string]interface{}
	triggerBootstrap func() error
}

type progressReader struct {
	r      io.Reader
	ingest map[string]interface{}
	mu     *sync.RWMutex
	read   int64
	s      *Server
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.r.Read(p)
	if n > 0 {
		pr.read += int64(n)
		pr.mu.Lock()
		pr.ingest["downloaded_chunks"] = int(pr.read / int64(cas.ChunkSize))
		// Create safe copy for broadcast
		payload := make(map[string]interface{})
		for k, v := range pr.ingest { payload[k] = v }
		pr.mu.Unlock()
		if pr.s != nil {
			pr.s.PublishEvent("cas_chunk_progress", payload)
		}
	}
	if err == io.EOF {
		pr.mu.Lock()
		if total, ok := pr.ingest["total_chunks"].(int); ok && total > 0 {
			pr.ingest["downloaded_chunks"] = total
		}
		// Create safe copy for broadcast
		payload := make(map[string]interface{})
		for k, v := range pr.ingest { payload[k] = v }
		pr.mu.Unlock()
		if pr.s != nil {
			pr.s.PublishEvent("cas_chunk_progress", payload)
		}
	}
	return n, err
}

func New(port int, host host.Host, startTime time.Time, store *store.Store, casStore *cas.Store, pub Publisher, fetcher Fetcher, bootstrapCb func() error) *Server {
	mux := http.NewServeMux()
	
	s := &Server{
		port:             port,
		nodeID:           host.ID().String(),
		startTime:        startTime,
		host:             host,
		store:            store,
		cas:              casStore,
		publisher:        pub,
		fetcher:          fetcher,
		eventBus:         NewEventBus(),
		activeIngestions: make(map[string]map[string]interface{}),
		triggerBootstrap: bootstrapCb,
	}

	// Basic endpoints
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/peers/connect", s.handleConnectPeer)
	mux.HandleFunc("/api/export", s.handleExportRVX)
	mux.HandleFunc("/api/inspect", s.handleInspectRVX)
	mux.HandleFunc("/api/import/rvx", s.handleExecuteRVX)
	mux.HandleFunc("/api/catalogs", s.handleCatalogs)
	mux.HandleFunc("/api/catalogs/", s.handleCatalogContents)
	mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("/api/bundles", s.handleBundles)
	mux.HandleFunc("/api/bundles/", s.handleBundleContents)
	mux.HandleFunc("/api/files/", s.handleFileComments)
	mux.HandleFunc("/api/archives/", s.handleArchiveView)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/dm", s.handleDM)
	mux.HandleFunc("/api/fetch", s.handleFetchFile)
	mux.HandleFunc("/api/downloads", s.handleDownloads)
	mux.HandleFunc("/api/import/url", s.handleImportURL)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/verify", s.handleVerifyFile)

	// Serve static frontend assets
	fs := http.FileServer(http.Dir("web/dist"))
	mux.Handle("/", fs)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler:mux,
	}

	// Telemetry Background Ticker
	go func() {
		ticker := time.NewTicker(2500 * time.Millisecond)
		defer ticker.Stop()
		for {
			<-ticker.C
			payload := s.buildPeersPayload()
			s.PublishEvent("mesh_state", payload)
		}
	}()

	return s
}

func (s *Server) Start() error {
	log.Printf("Starting local API server on http://127.0.0.1:%d", s.port)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server error: %w", err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	log.Println("Stopping API server...")
	return s.srv.Shutdown(ctx)
}

func (s *Server) PublishEvent(eventType string, payload interface{}) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventType, payload)
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	conf, _ := s.store.GetConfig()
	nodeName := ""
	if conf != nil && conf.NodeName != "" {
		nodeName = conf.NodeName
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "online",
		"version":        "0.1.0",
		"node_id":        s.nodeID,
		"node_name":      nodeName,
		"uptime_seconds": int(time.Since(s.startTime).Seconds()),
	})
}

func (s *Server) buildPeersPayload() []map[string]string {
	peers, err := s.store.GetPeers()
	if err != nil {
		return []map[string]string{}
	}

	conf, _ := s.store.GetConfig()
	retentionSecs := int64(120)
	if conf != nil && conf.PeerRetention > 0 {
		retentionSecs = int64(conf.PeerRetention)
	}

	now := time.Now().Unix()

	var response []map[string]string
	for _, p := range peers {
		route := "Unknown"
		if p.Multiaddr != "" {
			if strings.Contains(p.Multiaddr, "/quic") || strings.Contains(p.Multiaddr, "/udp") {
				route = "Direct (QUIC)"
			} else if strings.Contains(p.Multiaddr, "/tcp") {
				route = "Direct (TCP)"
			} else if strings.Contains(p.Multiaddr, "/ws") || strings.Contains(p.Multiaddr, "/wss") {
				route = "WebRTC/WS"
			} else if strings.Contains(p.Multiaddr, "/ble") {
				route = "Bluetooth (BLE)"
			} else if strings.Contains(p.Multiaddr, "/lora") {
				route = "Radio (LoRa)"
			} else {
				route = p.Multiaddr
			}
		}
		trust := "Unknown"
		if p.TrustLevel > 0 {
			trust = "Trusted"
		} else if strings.Contains(p.Multiaddr, "127.0.0.1") || strings.Contains(p.Multiaddr, "192.168.") {
			trust = "Verified" // Local net is explicitly verified
		}

		status := "Offline"
		latency := "N/A"
		if s.host != nil {
			pid, decodeErr := peer.Decode(p.ID)
			if decodeErr == nil {
				if s.host.Network().Connectedness(pid) == network.Connected {
					status = "Active"
					p.LastSeen = now
					go s.store.InsertPeer(p) // bump LastSeen asynchronously

					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					ch := ping.Ping(ctx, s.host, pid)
					res := <-ch
					cancel()
					if res.Error == nil {
						latency = fmt.Sprintf("%dms", res.RTT.Milliseconds())
					} else {
						latency = "Timeout"
					}
				}
			}
		}

		if status != "Active" && (now-p.LastSeen > retentionSecs) {
			continue
		}

		response = append(response, map[string]string{
			"id":      p.ID,
			"name":    p.Name,
			"status":  status,
			"latency": latency,
			"route":   route,
			"trust":   trust,
		})
	}

	if response == nil {
		response = []map[string]string{}
	}
	return response
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	payload := s.buildPeersPayload()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// handleConnectPeer accepts a JSON body {"multiaddr": "/ip4/.../p2p/..."} and manually dials the remote node.
func (s *Server) handleConnectPeer(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Multiaddr string `json:"multiaddr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	maddr, err := multiaddr.NewMultiaddr(req.Multiaddr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid multiaddr: %v", err), http.StatusBadRequest)
		return
	}

	pi, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid peer multiaddr formatting: %v", err), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := s.host.Connect(ctx, *pi); err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect: %v", err), http.StatusInternalServerError)
		return
	}

	// Persist the manually added peer
	now := time.Now().Unix()
	s.store.InsertPeer(&store.Peer{
		ID:         pi.ID.String(),
		Multiaddr:  maddr.String(),
		LastSeen:   now,
		TrustLevel: 100, // Manually added peers are highly trusted
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "connected", "peer_id": pi.ID.String()})
}

func (s *Server) handleCatalogs(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	if r.Method == http.MethodGet {
		catalogs, err := s.store.GetCatalogs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(catalogs)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		c := &store.Catalog{
			ID:          uuid.New().String(),
			Name:        req.Name,
			Description: req.Description,
			CreatedAt:   time.Now().Unix(),
		}
		if err := s.store.InsertCatalog(c); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.triggerBootstrap != nil {
		if err := s.triggerBootstrap(); err != nil {
			http.Error(w, "Bootstrap failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *Server) handleCatalogContents(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	path := r.URL.Path
	catalogID := filepath.Base(path)

	bundles, err := s.store.GetBundlesForCatalog(catalogID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := s.store.GetFilesForCatalog(catalogID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bundles": bundles,
		"files":   files,
	})
}

func (s *Server) handleBundles(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	if r.Method == http.MethodPost {
		var req struct {
			CatalogID      string `json:"catalog_id"`
			ParentBundleID string `json:"parent_bundle_id"`
			Type           string `json:"type"`
			Name           string `json:"name"`
			Description    string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		bundleType := req.Type
		if bundleType == "" {
			bundleType = "bundle"
		}

		b := &store.Bundle{
			ID:             uuid.New().String(),
			CatalogID:      req.CatalogID,
			ParentBundleID: req.ParentBundleID,
			Type:           bundleType,
			Name:           req.Name,
			Description:    req.Description,
			CreatedAt:      time.Now().Unix(),
		}
		if err := s.store.InsertBundle(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleBundleContents(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	path := r.URL.Path
	bundleID := filepath.Base(path)

	bundles, err := s.store.GetBundlesForBundle(bundleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := s.store.GetFilesForBundle(bundleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bundles": bundles,
		"files":   files,
	})
}

func (s *Server) handleImportURL(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileID    string `json:"file_id"`
		CatalogID string `json:"catalog_id"`
		BundleID  string `json:"bundle_id"`
		URL       string `json:"url"`
		Title     string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.CatalogID == "" {
		http.Error(w, "catalog_id is required", http.StatusBadRequest)
		return
	}

	// We run the huge payload stream ingestion asynchronously
	go func(fileID, urlStr, catId, bundleId, title string) {
		log.Printf("Starting stream ingestion for %s...", urlStr)

		if fileID == "" {
			fileID = uuid.New().String()
		}
		fileName := filepath.Base(urlStr)
		if title == "" {
			title = fileName
		}

		ingest := map[string]interface{}{
			"file_id":           fileID,
			"file_name":         fileName + " (HTTP Ingest)",
			"total_chunks":      -1,
			"downloaded_chunks": 0,
			"status":            "downloading",
			"start_time":        time.Now().Unix(),
			"peers":             []string{"HTTP Origin Server"},
		}

		s.ingestMu.Lock()
		s.activeIngestions[fileID] = ingest
		s.ingestMu.Unlock()

		defer func() {
			s.ingestMu.Lock()
			if ingest["status"] == "downloading" {
				ingest["status"] = "completed"
			}
			s.ingestMu.Unlock()
			
			go func() {
				time.Sleep(30 * time.Second)
				s.ingestMu.Lock()
				delete(s.activeIngestions, fileID)
				s.ingestMu.Unlock()
			}()
		}()

		resp, err := http.Get(urlStr)
		if err != nil || resp.StatusCode != http.StatusOK {
			s.ingestMu.Lock()
			ingest["status"] = "failed"
			if err != nil {
				ingest["error"] = err.Error()
			} else {
				ingest["error"] = "HTTP " + strconv.Itoa(resp.StatusCode)
			}
			s.ingestMu.Unlock()
			log.Printf("Ingestion failed: bad status or error from url %s", urlStr)
			return
		}
		defer resp.Body.Close()

		totalChunks := -1
		if resp.ContentLength > 0 {
			totalChunks = int(resp.ContentLength / int64(cas.ChunkSize))
			if resp.ContentLength%int64(cas.ChunkSize) != 0 {
				totalChunks++
			}
			s.ingestMu.Lock()
			ingest["total_chunks"] = totalChunks
			s.ingestMu.Unlock()
		}

		reader := &progressReader{
			r:      resp.Body,
			ingest: ingest,
			mu:     &s.ingestMu,
			s:      s,
		}

		// Write to CAS
		hashes, size, err := s.cas.IngestStream(reader)
		if err != nil {
			s.ingestMu.Lock()
			ingest["status"] = "failed"
			s.ingestMu.Unlock()
			log.Printf("Ingestion failed: failed to chunk file into cas: %v", err)
			return
		}

		hashBytes, _ := json.Marshal(hashes)

		f := &store.File{
			ID:          fileID,
			CatalogID:   catId,
			BundleID:    bundleId,
			Title:       title,
			Path:        fileName,
			Size:        size,
			ChunkHashes: string(hashBytes),
			SourceURL:   urlStr,
		}

		if err := s.store.InsertFile(f); err != nil {
			log.Printf("Ingestion failed: failed to link file in database: %v", err)
			return
		}
		log.Printf("Successfully ingested %s! Extracted %d chunks.", urlStr, len(hashes))
	}(req.FileID, req.URL, req.CatalogID, req.BundleID, req.Title)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "ingestion_started"})
}

func (s *Server) handleFileComments(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	var fileID string
	if _, err := fmt.Sscanf(path, "/api/files/%s/comments", &fileID); err != nil {
		fileID = path[len("/api/files/"):]
	}

	messages, err := s.store.GetSocialMessagesByRef(fileID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		messages, err := s.store.GetSocialMessages("/resilient/chat/global", 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Content     string `json:"content"`
			RefTargetID string `json:"ref_target_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if s.publisher != nil {
			msg, err := s.publisher.Publish(req.Content, req.RefTargetID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(msg)
		} else {
			http.Error(w, "Publisher not configured", http.StatusServiceUnavailable)
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleDM(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		recipientID := r.URL.Query().Get("peer_id")
		if recipientID == "" {
			http.Error(w, "peer_id required", http.StatusBadRequest)
			return
		}

		// Wait, recipient_id logic is a bit manual here. 
		// Actually, GetSocialMessagesByRef checks ref_target_id. 
		// We should add a specific DB query for DMs, or just use the existing one if we stored recipient_id there?
		// We stored it in recipient_id column. We should query where recipient_id = target OR author_id = target
		// For simplicity, let's just write the raw query here:
		rows, err := s.store.GetPeers() // Just a hack to check DB connection
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = rows
		
		// Proper query for a DM thread context
		dbRows, err := s.store.GetSocialMessages("direct", 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		// Filter for this specific peer manually for now to save a new DB method
		var filtered []*store.SocialMessage
		for _, m := range dbRows {
			if m.RecipientID == recipientID || m.AuthorID == recipientID {
				filtered = append(filtered, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(filtered)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Content     string `json:"content"`
			RecipientID string `json:"recipient_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if s.publisher != nil {
			msg, err := s.publisher.PublishDirect(req.Content, req.RecipientID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(msg)
		} else {
			http.Error(w, "Publisher not configured", http.StatusServiceUnavailable)
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleFetchFile(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileID string `json:"file_id"`
		PeerID string `json:"peer_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.fetcher == nil {
		http.Error(w, "Fetcher not configured", http.StatusServiceUnavailable)
		return
	}

	targetFile, err := s.store.GetFileByID(req.FileID)
	if err != nil {
		http.Error(w, "File not found: "+err.Error(), http.StatusNotFound)
		return
	}
	
	if req.PeerID != "" {
		pid, err := peer.Decode(req.PeerID)
		if err != nil {
			http.Error(w, "invalid peer ID: "+err.Error(), http.StatusBadRequest)
			return
		}
		err = s.fetcher.FetchFile(r.Context(), pid, targetFile, s.PublishEvent)
	} else {
		// Swarm download from all known peers
		dbPeers, _ := s.store.GetPeers()
		var pids []peer.ID
		for _, dbp := range dbPeers {
			if dbp.ID != "" {
				if pid, err := peer.Decode(dbp.ID); err == nil {
					pids = append(pids, pid)
				}
			}
		}
		err = s.fetcher.FetchFileSwarm(r.Context(), pids, targetFile, s.PublishEvent)
	}

	if err != nil {
		http.Error(w, "Fetch failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *Server) handleVerifyFile(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f, err := s.store.GetFileByID(req.FileID)
	if err != nil {
		http.Error(w, "File not found: "+err.Error(), http.StatusNotFound)
		return
	}

	var hashes []string
	if err := json.Unmarshal([]byte(f.ChunkHashes), &hashes); err != nil {
		http.Error(w, "Invalid file chunk metadata", http.StatusInternalServerError)
		return
	}

	// We'll iterate through all required chunks and verify if 
	// a) they exist on disk, and 
	// b) their content cryptographically matches the SHA256 identifier
	totalChunks := len(hashes)
	if totalChunks == 0 {
		out := map[string]interface{}{"status": "valid", "corrupted": 0, "total": 0, "missing": 0}
		json.NewEncoder(w).Encode(out)
		return
	}

	missing := 0
	corrupted := 0

	for _, expectedHash := range hashes {
		data, err := s.cas.ReadChunk(expectedHash)
		if err != nil {
			missing++
			continue
		}
		
		// Cryptographically verify payload
		actualHash := sha256.Sum256(data)
		actualHashStr := hex.EncodeToString(actualHash[:])
		if actualHashStr != expectedHash {
			corrupted++
		}
	}

	status := "valid"
	if missing > 0 || corrupted > 0 {
		status = "corrupted"
	}

	out := map[string]interface{}{
		"status":    status,
		"corrupted": corrupted,
		"missing":   missing,
		"total":     totalChunks,
		"valid":     totalChunks - missing - corrupted,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	downloads := s.fetcher.GetActiveDownloads()
	if downloads == nil {
		downloads = make(map[string]interface{})
	}

	ingestions := make(map[string]interface{})
	s.ingestMu.RLock()
	for k, v := range s.activeIngestions {
		copied := make(map[string]interface{})
		for ik, iv := range v {
			copied[ik] = iv
		}
		ingestions[k] = copied
	}
	s.ingestMu.RUnlock()

	out := map[string]interface{}{
		"swarm":     downloads,
		"transfers": ingestions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		conf, err := s.store.GetConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(conf)
		return
	}

	if r.Method == http.MethodPost {
		var conf store.NodeConfig
		if err := json.NewDecoder(r.Body).Decode(&conf); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.store.SaveConfig(&conf); err != nil {
			http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(conf)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
