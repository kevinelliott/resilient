package zim

import (
	"bufio"
	"encoding/binary"
	"io"
)

func readUint8(f FileSource) uint8 {
	const byteLen = 1
	var arr [byteLen]byte
	f.Read(arr[:byteLen])
	return arr[0]
}

func readUint16(f FileSource) uint16 {
	const byteLen = 2
	var arr [byteLen]byte
	f.Read(arr[:byteLen])
	return binary.LittleEndian.Uint16(arr[:byteLen])
}

func readUint32(f FileSource) uint32 {
	const byteLen = 4
	var arr [byteLen]byte
	f.Read(arr[:byteLen])
	return binary.LittleEndian.Uint32(arr[:byteLen])
}

func readUint32R(r io.Reader) uint32 {
	const byteLen = 4
	var arr [byteLen]byte
	r.Read(arr[:byteLen])
	return binary.LittleEndian.Uint32(arr[:byteLen])
}

func readUint64(f FileSource) uint64 {
	const byteLen = 8
	var arr [byteLen]byte
	f.Read(arr[:byteLen])
	return binary.LittleEndian.Uint64(arr[:byteLen])
}

func readUint64R(r io.Reader) uint64 {
	const byteLen = 8
	var arr [byteLen]byte
	r.Read(arr[:byteLen])
	return binary.LittleEndian.Uint64(arr[:byteLen])
}

func seek(f FileSource, position int64) {
	f.Seek(position, 0)
}

func currentPosition(f FileSource) int64 {
	currentPos, _ := f.Seek(0, 1)
	return currentPos
}

func readSlice(f FileSource, byteLen int) ([]byte, error) {
	var buf = make([]byte, byteLen)
	var _, readErr = f.Read(buf)
	return buf, readErr
}

func readNullTerminatedSlice(f FileSource) []byte {
	const bufferSize = 256
	var prevFilePosition = currentPosition(f)
	var bufReader = bufio.NewReaderSize(f, bufferSize)
	var result, readBufErr = bufReader.ReadBytes(0)
	var dataLen int
	if readBufErr == nil {
		dataLen = len(result) - 1
	}
	seek(f, prevFilePosition+int64(dataLen+1))
	return result[:dataLen]
}

func readNullTerminatedString(f FileSource) string {
	return string(readNullTerminatedSlice(f))
}

func (z *File) urlPointerAtPos(position uint32) uint64 {
	seek(z.f, int64(z.header.urlPtrPos)+int64(position*8))
	return readUint64(z.f)
}

func (z *File) titlePointerAtPos(position uint32) uint64 {
	seek(z.f, int64(z.header.titlePtrPos)+int64(position*4))
	return z.urlPointerAtPos(readUint32(z.f))
}

func (z *File) clusterPointerAtPos(position uint32) uint64 {
	seek(z.f, int64(z.header.clusterPtrPos)+int64(position*8))
	return readUint64(z.f)
}
