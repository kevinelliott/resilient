package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

const ChunkSize = 1024 * 1024 // 1MB chunks

type Store struct {
	baseDir string
	enc     *zstd.Encoder
	dec     *zstd.Decoder
}

func New(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}
	
	dec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}

	return &Store{
		baseDir: baseDir,
		enc:     enc,
		dec:     dec,
	}, nil
}

// GetStoreDir returns the underlying directory on disk where CAS objects are saved
func (s *Store) GetStoreDir() string {
	return s.baseDir
}

// GetChunk mimics ReadChunk for external accessor interfaces
func (s *Store) GetChunk(hashStr string) ([]byte, error) {
	return s.ReadChunk(hashStr)
}

// WriteChunk compresses and writes a chunk of data, returning its SHA-256 hash
func (s *Store) WriteChunk(data []byte) (string, error) {
	// Compress the data
	compressed := s.enc.EncodeAll(data, make([]byte, 0, len(data)))

	// Hash the UNCOMPRESSED data to get the stable ID
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	// Subdivide into directories to avoid too many files in one place (e.g. objects/ab/cd...)
	dirPath := filepath.Join(s.baseDir, hashStr[:2], hashStr[2:4])
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	filePath := filepath.Join(dirPath, hashStr)
	
	// Fast path: if it already exists, don't rewrite it
	if _, err := os.Stat(filePath); err == nil {
		return hashStr, nil
	}

	if err := os.WriteFile(filePath, compressed, 0644); err != nil {
		return "", err
	}

	return hashStr, nil
}

// HasChunk checks if a chunk already exists locally to avoid downloading it again
func (s *Store) HasChunk(hashStr string) bool {
	if len(hashStr) < 4 {
		return false
	}
	filePath := filepath.Join(s.baseDir, hashStr[:2], hashStr[2:4], hashStr)
	_, err := os.Stat(filePath)
	return err == nil
}

// ReadChunk reads and decompresses a chunk by its hash
func (s *Store) ReadChunk(hashStr string) ([]byte, error) {
	if len(hashStr) < 4 {
		return nil, fmt.Errorf("invalid hash length")
	}

	filePath := filepath.Join(s.baseDir, hashStr[:2], hashStr[2:4], hashStr)
	
	compressed, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	decompressed, err := s.dec.DecodeAll(compressed, nil)
	if err != nil {
		return nil, err
	}

	// Verify the hash
	actualHash := sha256.Sum256(decompressed)
	actualHashStr := hex.EncodeToString(actualHash[:])
	
	if actualHashStr != hashStr {
		return nil, fmt.Errorf("hash mismatch: expected %s, got %s", hashStr, actualHashStr)
	}

	return decompressed, nil
}

// IngestFile chunks a file from disk, stores all chunks in CAS, and returns the slice of chunk hashes
func (s *Store) IngestFile(filePath string) ([]string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}

	var hashes []string
	buf := make([]byte, ChunkSize)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h, err := s.WriteChunk(buf[:n])
			if err != nil {
				return nil, 0, err
			}
			hashes = append(hashes, h)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
	}

	return hashes, stat.Size(), nil
}

// IngestStream reads from an io.Reader, stores all chunks in CAS, and returns the slice of chunk hashes
func (s *Store) IngestStream(r io.Reader) ([]string, int64, error) {
	var hashes []string
	buf := make([]byte, ChunkSize)
	var totalSize int64

	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			h, writErr := s.WriteChunk(buf[:n])
			if writErr != nil {
				return nil, totalSize, writErr
			}
			hashes = append(hashes, h)
			totalSize += int64(n)
		}
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			break
		}
		if err != nil {
			return nil, totalSize, err
		}
	}

	return hashes, totalSize, nil
}

// AssembleFile reconstitutes a file from a list of chunk hashes
func (s *Store) AssembleFile(hashes []string, outPath string) error {
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, h := range hashes {
		data, err := s.ReadChunk(h)
		if err != nil {
			return fmt.Errorf("failed to read chunk %s: %w", h, err)
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
	}

	return nil
}
