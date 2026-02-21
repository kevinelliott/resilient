// Package zim implements reading support for the ZIM File Format.
package zim

import (
	"crypto/md5"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/xi2/xz"
)

// Some useful constants belonging to a ZIM file.
const (
	MagicNumber  = uint32(72173914)
	NoMainPage   = ^uint32(0)
	NoLayoutPage = NoMainPage
)

type FileSource interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

// File represents a ZIM file and contains the most important
// information that is retrieved once and used again.
type File struct {
	f            FileSource
	xzReader     *xz.Reader
	zstdReader   *zstd.Decoder
	header       Header
	metadata     map[string]string
	mimetypeList []string
}

// Open opens the file and checks for a valid ZIM header.
func Open(filename string) (*File, error) {
	var f, fileErr = os.Open(filename)
	if fileErr != nil {
		return nil, fileErr
	}
	return NewReader(f)
}

// NewReader opens a ZIM from a FileSource natively in memory or over a remote abstraction
func NewReader(f FileSource) (*File, error) {
	var xzReader, xzReaderErr = xz.NewReader(nil, 0)
	if xzReaderErr != nil {
		return nil, xzReaderErr
	}
	zstdReader, zstdReaderErr := zstd.NewReader(nil)
	if zstdReaderErr != nil {
		return nil, zstdReaderErr
	}
	var result = &File{
		f:          f,
		xzReader:   xzReader,
		zstdReader: zstdReader,
	}
	if headerErr := result.readHeader(); headerErr != nil {
		return nil, headerErr
	}
	result.readMimetypeList()
	result.readMetadata()
	return result, nil
}

// Close closes the ZIM file.
func (z *File) Close() {
	if z.zstdReader != nil {
		z.zstdReader.Close()
	}
	z.f.Close()
}

// ArticleCount is the total number of articles defined
// in the pointerlists of the ZIM file.
func (z *File) ArticleCount() uint32 {
	return z.header.articleCount
}

// ClusterCount is the number of clusters the ZIM file contains.
func (z *File) ClusterCount() uint32 {
	return z.header.clusterCount
}

// Filesize is the filesize in Bytes of the ZIM file.
func (z *File) Filesize() int {
	return int(z.header.checksumPos) + md5.Size
}

// UUID is the unique id of a ZIM file.
func (z *File) UUID() UUID {
	return z.header.uuid
}

// Version is the version tuple of the ZIM file.
func (z *File) Version() (majorVersion, minorVersion uint16) {
	return z.header.majorVersion, z.header.minorVersion
}
