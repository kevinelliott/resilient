package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"resilient/internal/cas"
	"resilient/internal/store"
)

const CASProtocolID = "/resilient/cas/1.0.0"

type ChunkRequest struct {
	Hash string `json:"hash"`
}

type ChunkResponse struct {
	Hash  string `json:"hash"`
	Found bool   `json:"found"`
	Data  []byte `json:"data,omitempty"`
}

type ActiveDownload struct {
	FileID           string `json:"file_id"`
	FileName         string `json:"file_name"`
	TotalChunks      int    `json:"total_chunks"`
	DownloadedChunks int    `json:"downloaded_chunks"`
	Status           string   `json:"status"` // "downloading", "completed", "failed"
	StartTime        int64    `json:"start_time"`
	Peers            []string `json:"peers"`
}

type CASSyncManager struct {
	ctx  context.Context
	h    host.Host
	cas  *cas.Store

	mu              sync.RWMutex
	activeDownloads map[string]*ActiveDownload
}

func setupCASSyncManager(ctx context.Context, h host.Host, cStore *cas.Store) *CASSyncManager {
	mgr := &CASSyncManager{
		ctx:             ctx,
		h:               h,
		cas:             cStore,
		activeDownloads: make(map[string]*ActiveDownload),
	}

	h.SetStreamHandler(CASProtocolID, mgr.handleStream)
	return mgr
}

func (mgr *CASSyncManager) GetActiveDownloads() map[string]interface{} {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	res := make(map[string]interface{})
	for k, v := range mgr.activeDownloads {
		res[k] = v // shallow copy of pointer is fine for JSON marshaling in our simple use case
	}
	return res
}

// handleStream responds to incoming chunk requests from peers
func (mgr *CASSyncManager) handleStream(s network.Stream) {
	defer s.Close()

	var req ChunkRequest
	if err := json.NewDecoder(s).Decode(&req); err != nil {
		log.Printf("Failed to read CAS chunk request: %v", err)
		return
	}

	res := ChunkResponse{Hash: req.Hash}
	data, err := mgr.cas.ReadChunk(req.Hash)
	if err == nil {
		res.Found = true
		res.Data = data
	}

	if err := json.NewEncoder(s).Encode(res); err != nil {
		log.Printf("Failed to write CAS chunk response: %v", err)
	}
}

// FetchChunk asks a specific peer for a chunk by its hash
func (mgr *CASSyncManager) FetchChunk(ctx context.Context, peerID peer.ID, hash string) ([]byte, error) {
	s, err := mgr.h.NewStream(ctx, peerID, CASProtocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream to peer %s: %w", peerID, err)
	}
	defer s.Close()

	req := ChunkRequest{Hash: hash}
	if err := json.NewEncoder(s).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send chunk request: %w", err)
	}

	var res ChunkResponse
	if err := json.NewDecoder(s).Decode(&res); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("peer closed stream before response")
		}
		return nil, fmt.Errorf("failed to read chunk response: %w", err)
	}

	if !res.Found {
		return nil, fmt.Errorf("chunk %s not found on peer %s", hash, peerID)
	}

	return res.Data, nil
}

// FetchFile attempts to download an entire file sequentially from a given peer
func (mgr *CASSyncManager) FetchFile(ctx context.Context, peerID peer.ID, file *store.File) error {
	var chunkHashes []string
	if err := json.Unmarshal([]byte(file.ChunkHashes), &chunkHashes); err != nil {
		return fmt.Errorf("invalid chunk hashes JSON: %w", err)
	}

	log.Printf("Starting download of file %s from peer %s (%d chunks)", file.ID, peerID.String(), len(chunkHashes))

	for i, hash := range chunkHashes {
		// Optimization: Check if we already have it
		if mgr.cas.HasChunk(hash) {
			continue // skip download
		}

		data, err := mgr.FetchChunk(ctx, peerID, hash)
		if err != nil {
			return fmt.Errorf("failed fetching chunk %d (%s): %w", i, hash, err)
		}

		// Validate chunk integrity cryptographically before saving
		actualHash := sha256.Sum256(data)
		actualHashStr := hex.EncodeToString(actualHash[:])
		if actualHashStr != hash {
			return fmt.Errorf("cryptographic mismatch: downloaded chunk %d hash %s does not match expected %s", i, actualHashStr, hash)
		}

		// Save string ID to CAS
		if _, err := mgr.cas.WriteChunk(data); err != nil {
			return fmt.Errorf("failed writing chunk to CAS: %w", err)
		}
	}

	log.Printf("Successfully downloaded all chunks for file %s", file.ID)
	return nil
}

// FetchFileSwarm attempts to download an entire file's chunks in parallel from a list of peers
func (mgr *CASSyncManager) FetchFileSwarm(ctx context.Context, peers []peer.ID, file *store.File) error {
	var chunkHashes []string
	if err := json.Unmarshal([]byte(file.ChunkHashes), &chunkHashes); err != nil {
		return fmt.Errorf("invalid chunk hashes JSON: %w", err)
	}

	log.Printf("Starting swarm download of file %s (%d chunks) from %d peers", file.ID, len(chunkHashes), len(peers))

	if len(peers) == 0 {
		return fmt.Errorf("no peers available for swarm download")
	}

	var peerStrings []string
	for _, p := range peers {
		peerStrings = append(peerStrings, p.String())
	}

	activeDL := &ActiveDownload{
		FileID:           file.ID,
		FileName:         file.Path,
		TotalChunks:      len(chunkHashes),
		DownloadedChunks: 0,
		Status:           "downloading",
		StartTime:        time.Now().Unix(),
		Peers:            peerStrings,
	}

	mgr.mu.Lock()
	mgr.activeDownloads[file.ID] = activeDL
	mgr.mu.Unlock()

	defer func() {
		mgr.mu.Lock()
		if activeDL.Status == "downloading" {
			activeDL.Status = "completed"
		}
		// In a real app we might clean up completed downloads after some time,
		// but for the UI we can let them stick around so the user sees "completed".
		mgr.mu.Unlock()
	}()

	// Simple concurrency: create a channel of chunks needed
	type job struct {
		index int
		hash  string
	}
	type result struct {
		index int
		hash  string
		data  []byte
		err   error
	}

	jobs := make(chan job, len(chunkHashes))
	results := make(chan result, len(chunkHashes))

	for i, hash := range chunkHashes {
		jobs <- job{index: i, hash: hash}
	}
	close(jobs)

	// Start a worker per peer
	for _, p := range peers {
		go func(peerID peer.ID) {
			for j := range jobs {
				if mgr.cas.HasChunk(j.hash) {
					results <- result{index: j.index, hash: j.hash, data: nil, err: nil} // already have it
					continue
				}

				data, err := mgr.FetchChunk(ctx, peerID, j.hash)
				results <- result{index: j.index, hash: j.hash, data: data, err: err}
			}
		}(p)
	}

	// Collect results and retry logic (simplified)
	successCount := 0
	for i := 0; i < len(chunkHashes); i++ {
		res := <-results
		if res.err != nil {
			log.Printf("Warning: failed to fetch chunk %s: %v", res.hash, res.err)
			// In a robust implementation, we would re-queue the job here
			// For simplicity we fail the swarm download if any chunk completely fails (since we only tried once per chunk via the worker queue)
			mgr.mu.Lock()
			activeDL.Status = "failed"
			mgr.mu.Unlock()
			return fmt.Errorf("failed fetching chunk %s: %w", res.hash, res.err)
		}

		if res.data != nil {
			// Validate chunk integrity cryptographically before saving
			actualHash := sha256.Sum256(res.data)
			actualHashStr := hex.EncodeToString(actualHash[:])
			if actualHashStr != res.hash {
				mgr.mu.Lock()
				activeDL.Status = "failed"
				mgr.mu.Unlock()
				return fmt.Errorf("cryptographic mismatch: downloaded chunk hash %s does not match expected %s", actualHashStr, res.hash)
			}

			if _, err := mgr.cas.WriteChunk(res.data); err != nil {
				mgr.mu.Lock()
				activeDL.Status = "failed"
				mgr.mu.Unlock()
				return fmt.Errorf("failed writing chunk to CAS: %w", err)
			}
		}
		successCount++
		mgr.mu.Lock()
		activeDL.DownloadedChunks = successCount
		mgr.mu.Unlock()
	}

	log.Printf("Successfully swarm downloaded %d chunks for file %s", successCount, file.ID)
	return nil
}
