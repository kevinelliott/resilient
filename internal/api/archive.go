package api

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/slyrz/warc"
	"resilient/internal/zim"
)

// handleArchiveView routes requests for a file ID and inner path
func (s *Server) handleArchiveView(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// URL format: /api/archives/<file_id>/<inner_path...>
	// E.g. /api/archives/uuid-1234/index.html
	prefix := "/api/archives/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	rest := r.URL.Path[len(prefix):]
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing file ID", http.StatusBadRequest)
		return
	}

	fileID := parts[0]
	innerPath := ""
	if len(parts) > 1 {
		innerPath = parts[1]
	}

	// 1. Fetch file record from DB
	f, err := s.store.GetFileByID(fileID)
	if err != nil {
		http.Error(w, "File not found: "+err.Error(), http.StatusNotFound)
		return
	}

	var hashes []string
	if err := json.Unmarshal([]byte(f.ChunkHashes), &hashes); err != nil {
		http.Error(w, "Invalid file chunk metadata", http.StatusInternalServerError)
		return
	}

	// 2. Set Content-Type based on extension of inner path
	switch {
	case strings.HasSuffix(innerPath, ".html"):
		w.Header().Set("Content-Type", "text/html")
	case strings.HasSuffix(innerPath, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(innerPath, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(innerPath, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(innerPath, ".jpg") || strings.HasSuffix(innerPath, ".jpeg"):
		w.Header().Set("Content-Type", "image/jpeg")
	case strings.HasSuffix(innerPath, ".txt") || strings.HasSuffix(f.Path, ".txt"):
		w.Header().Set("Content-Type", "text/plain")
	case strings.HasSuffix(innerPath, ".pdf") || strings.HasSuffix(f.Path, ".pdf"):
		w.Header().Set("Content-Type", "application/pdf")
	case strings.HasSuffix(innerPath, ".json") || strings.HasSuffix(f.Path, ".json"):
		w.Header().Set("Content-Type", "application/json")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// 3. Extract based on outer file extension
	if strings.HasSuffix(f.Path, ".zip") {
		s.streamZip(w, hashes, f.Size, innerPath)
		return
	} else if strings.HasSuffix(f.Path, ".tar") {
		s.streamTar(w, hashes, f.Size, innerPath)
		return
	} else if strings.HasSuffix(f.Path, ".zim") {
		s.streamZim(w, hashes, f.Size, innerPath)
		return
	} else if strings.HasSuffix(f.Path, ".warc") || strings.HasSuffix(f.Path, ".wacz") {
		s.streamWarc(w, hashes, f.Size, innerPath)
		return
	}

	// 4. Default File Bypass (Un-Archived Plaint-Text or Binary)
	vf := s.cas.NewVirtualFile(hashes, f.Size)
	io.Copy(w, vf)
}

func (s *Server) streamZip(w http.ResponseWriter, hashes []string, size int64, innerPath string) {
	// Our CAS VFS allows reading parts of the ZIP directly without downloading the whole thing
	vf := s.cas.NewVirtualFile(hashes, size)
	
	zr, err := zip.NewReader(vf, size)
	if err != nil {
		http.Error(w, "Error opening zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, zf := range zr.File {
		if zf.Name == innerPath {
			rc, err := zf.Open()
			if err != nil {
				http.Error(w, "Error reading inner file: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer rc.Close()

			io.Copy(w, rc)
			return
		}
	}

	http.Error(w, "File not found in archive", http.StatusNotFound)
}

func (s *Server) streamTar(w http.ResponseWriter, hashes []string, size int64, innerPath string) {
	vf := s.cas.NewVirtualFile(hashes, size)
	tr := tar.NewReader(vf)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break 
		}
		if err != nil {
			http.Error(w, "Error reading tar: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if hdr.Name == innerPath {
			io.Copy(w, tr)
			return
		}
	}

	http.Error(w, "File not found in archive", http.StatusNotFound)
}

func (s *Server) streamZim(w http.ResponseWriter, hashes []string, size int64, innerPath string) {
	vf := s.cas.NewVirtualFile(hashes, size)
	
	zr, err := zim.NewReader(vf)
	if err != nil {
		http.Error(w, "Error opening zim: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var entry zim.DirectoryEntry
	var found bool

	// If no inner path provided, route directly to the ZIM main page
	if innerPath == "" {
		mainPage, err := zr.MainPage()
		if err == nil {
			entry = mainPage
			found = true
			innerPath = string(mainPage.URL())
		}
	}

	// Attempt to find the entry across common namespaces if we don't have it yet
	if !found {
		// If the path already has a namespace prefix (e.g. 'A/something.html' or 'I/image.png')
		// We check if it matches exactly first by parsing the first character.
		if len(innerPath) > 2 && innerPath[1] == '/' {
			ns := zim.Namespace(innerPath[0])
			entry, _, found = zr.EntryWithURL(ns, []byte(innerPath[2:]))
		}

		if !found {
			namespacesToTry := []zim.Namespace{
				zim.NamespaceArticles, 
				zim.NamespaceLayout, 
				zim.NamespaceImagesFiles, 
				zim.NamespaceImagesText,
			}
			for _, ns := range namespacesToTry {
				entry, _, found = zr.EntryWithURL(ns, []byte(innerPath))
				if found {
					break
				}
			}
		}
	}

	if !found {
		http.Error(w, "File not found in ZIM archive", http.StatusNotFound)
		return
	}

	if entry.IsRedirect() {
		// Follow redirect once
		redirectEntry, err := zr.FollowRedirect(&entry)
		if err == nil {
			entry = redirectEntry
		}
	}

	rc, _, err := zr.BlobReader(&entry)
	if err != nil {
		http.Error(w, "Failed to read ZIM blob: "+err.Error(), http.StatusInternalServerError)
		return
	}

	io.Copy(w, rc)
}

func (s *Server) streamWarc(w http.ResponseWriter, hashes []string, size int64, innerPath string) {
	vf := s.cas.NewVirtualFile(hashes, size)

	var reader io.Reader = vf

	// If it is a WACZ, it is actually a zip file where the warcs are inside the archive/ dir.
	// We'll need to extract the WARC from the zip first. If so, just grab the first .warc/.warc.gz we see.
	// For a pure .warc or .warc.gz, we just pass the VF directly.
	// However, WACZ might have multiple WARCs and an index. For a simpler streaming implementaton,
	// let's try to pass the raw WARC data to slyrz/warc.
	
	// This simple streamWarc tries to iterate through the records and match the target URL (innerPath).
	// For production we'd want to parse CDX/idx files, but for the basic streaming API:

	warcReader, err := warc.NewReader(reader)
	if err != nil {
		http.Error(w, "Failed to initialize WARC reader: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer warcReader.Close()

	// Normalize target URL (sometimes WARCs use full URLs, our innerPath might be just the end)
	// Example: innerPath might be "http://example.com/index.html"
	
	for {
		record, err := warcReader.ReadRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Some records may be malformed, skip them
			continue
		}

		targetURI := record.Header.Get("WARC-Target-URI")
		if targetURI != "" {
			// See if the requested inner path matches the Target URI
			// In many archives, the user might just request "index.html" and we'd have to guess.
			// But if they request the full URL, we match it.
			if strings.HasSuffix(targetURI, innerPath) && record.Header.Get("WARC-Type") == "response" {
				
				// Strip HTTP headers from WARC content (WARC 'response' records store the raw HTTP response, including headers)
				// We can just dump the raw content for this naive implementation, but ideally we'd parse the HTTP block.
				// Since we just want to prove extraction works without CGO:
				io.Copy(w, record.Content)
				return
			}
		}
	}

	http.Error(w, "File not found in WARC/WACZ archive", http.StatusNotFound)
}
