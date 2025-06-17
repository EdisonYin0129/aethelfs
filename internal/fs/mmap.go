package fs

import (
	"aethelfs/pkg/cache"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/cpu"
	"golang.org/x/sys/unix"
)

// MmapFile represents a memory-mapped file.
type MmapFile struct {
	File   *os.File
	Length int
	Data   []byte
}

// Mmap creates a memory mapping for the specified file.
func Mmap(file *os.File, length int) (*MmapFile, error) {
	data, err := unix.Mmap(int(file.Fd()), 0, length, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	return &MmapFile{
		File:   file,
		Length: length,
		Data:   data,
	}, nil
}

// Unmap unmaps the memory-mapped file.
func (m *MmapFile) Unmap() error {
	if err := unix.Munmap(m.Data); err != nil {
		return err
	}
	return nil
}

// Flush flushes the memory-mapped data to the underlying device.
// Optionally, this can use msync() or CLWB for flushing.
func (m *MmapFile) Flush() error {
	// Try to use CLWB first if supported by CPU
	if useCLWBIfAvailable(m.Data) {
		// If CLWB was successful, we can return early
		// CLWB is much faster than msync
		return nil
	}

	// Fall back to msync if CLWB is not available or failed
	if err := unix.Msync(m.Data, unix.MS_SYNC); err != nil {
		return err
	}
	return nil
}

// FlushRegion flushes a specific region of the memory-mapped data
func (m *MmapFile) FlushRegion(offset, length int) error {
	// Bounds check
	if offset < 0 || offset+length > len(m.Data) {
		return fmt.Errorf("flush region out of bounds: offset=%d, length=%d, size=%d",
			offset, length, len(m.Data))
	}

	// Try CLWB first for the specific region
	regionData := m.Data[offset : offset+length]
	if useCLWBIfAvailable(regionData) {
		return nil
	}

	// Fall back to msync for the region
	// Ensure alignment to page boundaries to avoid EINVAL
	pageSize := os.Getpagesize()
	alignedOffset := (offset / pageSize) * pageSize
	alignedEnd := ((offset + length + pageSize - 1) / pageSize) * pageSize

	if alignedEnd > len(m.Data) {
		alignedEnd = len(m.Data)
	}

	alignedRegion := m.Data[alignedOffset:alignedEnd]
	return unix.Msync(alignedRegion, unix.MS_SYNC)
}

// useCLWBIfAvailable attempts to use CLWB if the CPU supports it
// Returns true if CLWB was used successfully
func useCLWBIfAvailable(data []byte) bool {
	if len(data) == 0 {
		return true // Nothing to flush
	}

	// Only check for SSE2 support since that's what we use in our cache package
	if cpu.X86.HasSSE2 {
		// Get pointer to the start of the data
		ptr := unsafe.Pointer(&data[0])
		// Use cache package to flush each cache line
		cache.EnsureDataConsistency(ptr, len(data))
		return true
	}

	return false // CPU doesn't support needed cache flushing instructions
}
