package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"resilient/internal/store"
)

func (s *Server) handleInspectRVX(w http.ResponseWriter, r *http.Request) {
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
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	// 1. First Range Request (0-256) to read Magic + Length
	hReq, err := http.NewRequest("GET", req.URL, nil)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	hReq.Header.Set("Range", "bytes=0-256")
	resp, err := http.DefaultClient.Do(hReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch RVX: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Server does not support partial content: %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	headBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read headers", http.StatusInternalServerError)
		return
	}

	lines := strings.SplitN(string(headBytes), "\n", 3)
	if len(lines) < 2 || lines[0] != "---RVX-EXPORT-V1---" {
		http.Error(w, "Invalid RVX signature", http.StatusBadRequest)
		return
	}

	headerLength, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		http.Error(w, "Failed to parse RVX header size", http.StatusBadRequest)
		return
	}

	// Calculate exact bounds
	magicLen := int64(len(lines[0]) + 1) // +1 for '\n'
	sizeLen := int64(len(lines[1]) + 1)
	jsonStart := magicLen + sizeLen
	jsonEnd := jsonStart + headerLength - 1

	// 2. Second Range Request for precise JSON map
	mReq, err := http.NewRequest("GET", req.URL, nil)
	mReq.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", jsonStart, jsonEnd))
	mResp, err := http.DefaultClient.Do(mReq)
	if err != nil {
		http.Error(w, "Failed to fetch metadata block", http.StatusBadGateway)
		return
	}
	defer mResp.Body.Close()

	jsonBytes, err := io.ReadAll(mResp.Body)
	if err != nil {
		http.Error(w, "Failed to read JSON map", http.StatusInternalServerError)
		return
	}

	var meta RVXHeader
	if err := json.Unmarshal(jsonBytes, &meta); err != nil {
		http.Error(w, "Invalid header JSON structure", http.StatusBadRequest)
		return
	}

	// 3. Mesh Availability Check
	availableChunks := 0
	for _, h := range meta.ChunkHashes {
		// Check local CAS first
		if _, err := os.Stat(s.cas.GetStoreDir() + "/" + h[:2] + "/" + h); err == nil {
			availableChunks++
			continue
		}
		// In a real Kademlia check, we'd fire a DHT Provide query here.
		// For UI mockup speed, we mock it based on connected peers if we have any.
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"header": meta,
		"mesh_availability": map[string]interface{}{
			"total_chunks": len(meta.ChunkHashes),
			"local_chunks": availableChunks,
			"peer_chunks":  0, // Mock for now until DHT integration deepens
		},
	})
}

func (s *Server) handleExecuteRVX(w http.ResponseWriter, r *http.Request) {
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
		URL      string      `json:"url"`
		Header   RVXHeader   `json:"header"`
		Strategy string      `json:"strategy"` // http, mesh, hybrid
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Reconstruct Database Records from Header
	var newFiles []*store.File
	rawMeta, _ := json.Marshal(req.Header.Metadata)
	rawContents, _ := json.Marshal(req.Header.Contents)

	switch req.Header.Type {
	case "catalog":
		var c store.Catalog
		json.Unmarshal(rawMeta, &c)
		s.store.InsertCatalog(&c)

		// Top level items
		var items []map[string]interface{}
		json.Unmarshal(rawContents, &items)
		for _, item := range items {
			if item["type"] == "file" {
				var f store.File
				itemBytes, _ := json.Marshal(item)
				json.Unmarshal(itemBytes, &f)
				f.CatalogID = c.ID
				hashBytes, _ := json.Marshal(item["chunk_hashes"])
				f.ChunkHashes = string(hashBytes)
				s.store.InsertFile(&f)
				newFiles = append(newFiles, &f)
			} else {
				var b store.Bundle
				itemBytes, _ := json.Marshal(item)
				json.Unmarshal(itemBytes, &b)
				b.CatalogID = c.ID
				s.store.InsertBundle(&b)
			}
		}
	case "bundle":
		var b store.Bundle
		json.Unmarshal(rawMeta, &b)
		s.store.InsertBundle(&b)
	case "file":
		var f store.File
		json.Unmarshal(rawMeta, &f)
		
		hashBytes, _ := json.Marshal(req.Header.ChunkHashes)
		f.ChunkHashes = string(hashBytes)
		s.store.InsertFile(&f)
		newFiles = append(newFiles, &f)
	}

	// 2. Execute Chunk Retrieval Engine
	go func() {
		log.Printf("[RVX Ingest] Starting engine using '%s' strategy...", req.Strategy)
		
		if req.Strategy == "mesh" || req.Strategy == "hybrid" {
			// Trigger Swarm Fetchers
			if s.fetcher != nil {
				dbPeers, _ := s.store.GetPeers()
				var pids []peer.ID
				for _, dbp := range dbPeers {
					if pid, err := peer.Decode(dbp.ID); err == nil && dbp.ID != "" {
						pids = append(pids, pid)
					}
				}
				for _, nf := range newFiles {
					go s.fetcher.FetchFileSwarm(r.Context(), pids, nf, s.PublishEvent)
				}
			}
		}

		if req.Strategy == "http" || req.Strategy == "hybrid" {
			log.Printf("[RVX Ingest] Connecting to HTTP stream: %s", req.URL)
			// TODO: Add HTTP block-aware byte range downloader.
			// Falling back to GUI alerts for MVP completion.
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "ingestion_started"})
}
