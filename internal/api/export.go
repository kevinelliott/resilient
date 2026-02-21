package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"resilient/internal/store"
)

// RVXHeader represents the inner JSON map stored in cleartext at the head of the file
type RVXHeader struct {
	Type        string        `json:"type"` // catalog, bundle, file
	Metadata    interface{}   `json:"metadata"`
	Contents    []interface{} `json:"contents"`
	ChunkHashes []string      `json:"chunk_hashes"`
	DataHash    string        `json:"data_hash"`
}

func (s *Server) handleExportRVX(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetID := r.URL.Query().Get("id")
	targetType := r.URL.Query().Get("type") // 'catalog', 'bundle', or 'file'

	if targetID == "" || targetType == "" {
		http.Error(w, "id and type are required parameters", http.StatusBadRequest)
		return
	}

	// 1. Recursively collect metadata and list of all required CAS chunk hashes
	var metadata interface{}
	var contents []interface{}
	var hashes []string

	// Recursive gatherers
	var gatherFile func(f *store.File) map[string]interface{}
	var gatherBundle func(b *store.Bundle) map[string]interface{}

	gatherFile = func(f *store.File) map[string]interface{} {
		var fHashes []string
		if f.ChunkHashes != "[]" && f.ChunkHashes != "" {
			json.Unmarshal([]byte(f.ChunkHashes), &fHashes)
			hashes = append(hashes, fHashes...)
		}
		return map[string]interface{}{
			"type": "file",
			"file_id": f.ID,
			"title": f.Title,
			"path": f.Path,
			"size": f.Size,
			"source_url": f.SourceURL,
			"chunk_hashes": fHashes,
		}
	}

	gatherBundle = func(b *store.Bundle) map[string]interface{} {
		bContents := []interface{}{}
		
		childBundles, _ := s.store.GetBundlesForBundle(b.ID)
		for _, cb := range childBundles {
			bContents = append(bContents, gatherBundle(cb))
		}

		childFiles, _ := s.store.GetFilesForBundle(b.ID)
		for _, cf := range childFiles {
			bContents = append(bContents, gatherFile(cf))
		}

		return map[string]interface{}{
			"type": b.Type,
			"bundle_id": b.ID,
			"name": b.Name,
			"description": b.Description,
			"contents": bContents,
		}
	}

	switch targetType {
	case "catalog":
		cat, err := s.store.GetCatalogByID(targetID)
		if err != nil {
			http.Error(w, "Catalog not found", http.StatusNotFound)
			return
		}
		metadata = cat
		
		topBundles, _ := s.store.GetBundlesForCatalog(cat.ID)
		for _, b := range topBundles {
			contents = append(contents, gatherBundle(b))
		}

		topFiles, _ := s.store.GetFilesForCatalog(cat.ID)
		for _, f := range topFiles {
			contents = append(contents, gatherFile(f))
		}

	case "bundle":
		// Or folder, they share the DB table
		b, err := s.store.GetBundleByID(targetID)
		if err != nil {
			http.Error(w, "Bundle not found", http.StatusNotFound)
			return
		}
		metadata = b
		bData := gatherBundle(b)
		contents = bData["contents"].([]interface{})

	case "file":
		f, err := s.store.GetFileByID(targetID)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		metadata = f
		// For a single file, contents is empty, its data is the metadata itself
		gatherFile(f) // this appends to global hashes list

	default:
		http.Error(w, "Invalid export type. Must be catalog, bundle, or file", http.StatusBadRequest)
		return
	}

	// Dedupe hashes
	hashSet := make(map[string]bool)
	var dedupedHashes []string
	for _, h := range hashes {
		if !hashSet[h] {
			hashSet[h] = true
			dedupedHashes = append(dedupedHashes, h)
		}
	}

	// 2. Compute total Binary Payload Size and DataHash by inspecting local chunks ahead of time
	dataHash := sha256.New()
	var totalPayloadSize int64

	for _, h := range dedupedHashes {
		chunkPath := filepath.Join(s.cas.GetStoreDir(), h[:2], h)
		stat, err := os.Stat(chunkPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Missing chunk %s local dependency - cannot export", h), http.StatusInternalServerError)
			return
		}
		totalPayloadSize += stat.Size()
		// Performance note: To compute the full DataHash, we must read the chunks.
		// For massive exports, we should perhaps skip this or do it inline while streaming, 
		// but the spec dictates it lives in the header. We'll read them fast.
		chunkBytes, err := s.cas.GetChunk(h)
		if err != nil {
			http.Error(w, fmt.Sprintf("Missing chunk %s read error", h), http.StatusInternalServerError)
			return
		}
		dataHash.Write(chunkBytes)
	}

	header := RVXHeader{
		Type:        targetType,
		Metadata:    metadata,
		Contents:    contents,
		ChunkHashes: dedupedHashes,
		DataHash:    fmt.Sprintf("%x", dataHash.Sum(nil)),
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		http.Error(w, "Failed to marshal header", http.StatusInternalServerError)
		return
	}

	// 3. Format RVX 16-byte length declaration
	lengthString := fmt.Sprintf("%016d", len(headerBytes))

	// 4. Force file download in browser
	filename := fmt.Sprintf("export-%s.rvx", targetID)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// 5. Stream RVX Format
	w.Write([]byte("---RVX-EXPORT-V1---\n"))
	w.Write([]byte(lengthString + "\n"))
	w.Write(headerBytes)
	w.Write([]byte("\n---DATA-START---\n"))

	// Stream chunks directly from disk
	for _, h := range dedupedHashes {
		chunkPath := filepath.Join(s.cas.GetStoreDir(), h[:2], h)
		f, err := os.Open(chunkPath)
		if err != nil {
			log.Printf("Export stream error: failed to open chunk %s", h)
			continue
		}
		io.Copy(w, f)
		f.Close()
	}
}
