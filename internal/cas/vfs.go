package cas

import (
	"errors"
	"io"
)

// VirtualFile implements io.Reader, io.ReaderAt, and io.Seeker
// over a list of CAS chunk hashes, allowing massive files (like 100GB ZIMs)
// to be read and seeked without reconstituting them to disk.
type VirtualFile struct {
	store  *Store
	hashes []string
	size   int64
	offset int64
}

// NewVirtualFile creates a new VirtualFile from a list of chunk hashes and total size
func (s *Store) NewVirtualFile(hashes []string, size int64) *VirtualFile {
	return &VirtualFile{
		store:  s,
		hashes: hashes,
		size:   size,
		offset: 0,
	}
}

func (vf *VirtualFile) Read(p []byte) (n int, err error) {
	if vf.offset >= vf.size {
		return 0, io.EOF
	}

	n, err = vf.ReadAt(p, vf.offset)
	vf.offset += int64(n)
	return n, err
}

func (vf *VirtualFile) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = vf.offset + offset
	case io.SeekEnd:
		newOffset = vf.size + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if newOffset < 0 {
		return 0, errors.New("negative offset")
	}

	vf.offset = newOffset
	return newOffset, nil
}

func (vf *VirtualFile) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= vf.size {
		return 0, io.EOF
	}

	// Calculate which chunk we start in
	startChunkIdx := int(off / ChunkSize)
	chunkOff := int(off % ChunkSize)

	bytesRead := 0
	pLen := len(p)

	for idx := startChunkIdx; idx < len(vf.hashes) && bytesRead < pLen; idx++ {
		// Read the chunk
		chunkData, err := vf.store.ReadChunk(vf.hashes[idx])
		if err != nil {
			return bytesRead, err
		}

		// Calculate how much we can read from this chunk
		available := len(chunkData) - chunkOff
		if available <= 0 {
			break // Should not happen with well-formed chunks unless it's the end
		}

		needed := pLen - bytesRead
		toCopy := available
		if toCopy > needed {
			toCopy = needed
		}

		copy(p[bytesRead:], chunkData[chunkOff:chunkOff+toCopy])
		bytesRead += toCopy
		
		// For subsequent chunks, we start reading from the beginning of the chunk
		chunkOff = 0 
	}

	if bytesRead < pLen && vf.size <= off+int64(bytesRead) {
		return bytesRead, io.EOF
	}

	return bytesRead, nil
}

// Close implements io.Closer for the VirtualFile. It is essentially a no-op 
// because chunks are stateless and memory mapped/downloaded on-the-fly.
func (vf *VirtualFile) Close() error {
	return nil
}
