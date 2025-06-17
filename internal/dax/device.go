package dax

import (
	"aethelfs/internal/common"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Device represents a DAX character device
type Device struct {
	file     *os.File
	size     int64
	mmapData []byte
}

// NewDevice opens a DAX device and maps it into memory
func NewDevice(path string) (*Device, error) {
	// Check if the path exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("DAX device not found: %v", err)
	}

	// Open the device file
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open DAX device: %v", err)
	}

	// Get the size of the device
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat DAX device: %v", err)
	}
	size := stat.Size()

	// For DAX character devices, stat.Size() might be 0
	// In this case, use our configured maximum size
	if size <= 4096 {
		// Use configured maximum size from common package
		size = common.MaxFilesystemSize
		fmt.Printf("DAX device size unknown, using configured maximum: %d bytes (%.2f GB)\n",
			size, float64(size)/(1024*1024*1024))
	}

	// Memory map the device
	mmapData, err := unix.Mmap(int(file.Fd()), 0, int(size),
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap DAX device: %v", err)
	}

	return &Device{
		file:     file,
		size:     size,
		mmapData: mmapData,
	}, nil
}

// Size returns the size of the DAX device
func (d *Device) Size() int64 {
	return d.size
}

// MmapData returns the memory-mapped data
func (d *Device) MmapData() []byte {
	return d.mmapData
}

// Flush ensures all data is written to storage
func (d *Device) Flush() error {
	// Validate the data slice is not nil
	if d.mmapData == nil || len(d.mmapData) == 0 {
		return fmt.Errorf("no mapped data to flush")
	}

	// On some systems, msync can fail if the memory region is too large
	// Let's flush in smaller chunks to prevent this
	pageSize := os.Getpagesize()
	chunkSize := 64 * 1024 * 1024 // 64MB chunks

	// Only flush smaller chunks if the total size is very large
	if len(d.mmapData) > chunkSize*2 {
		var lastErr error

		for offset := 0; offset < len(d.mmapData); offset += chunkSize {
			end := offset + chunkSize
			if end > len(d.mmapData) {
				end = len(d.mmapData)
			}

			// Make sure we align to page boundaries
			alignedOffset := (offset / pageSize) * pageSize
			alignedEnd := ((end + pageSize - 1) / pageSize) * pageSize

			if alignedEnd > len(d.mmapData) {
				alignedEnd = len(d.mmapData)
			}

			// Skip empty chunks
			if alignedEnd <= alignedOffset {
				continue
			}

			chunk := d.mmapData[alignedOffset:alignedEnd]
			if err := unix.Msync(chunk, unix.MS_SYNC); err != nil {
				lastErr = fmt.Errorf("msync failed for chunk %d-%d: %w",
					alignedOffset, alignedEnd, err)
				// Continue with other chunks instead of returning immediately
			}
		}

		return lastErr
	}

	// For smaller regions, just do a single msync
	if err := unix.Msync(d.mmapData, unix.MS_SYNC); err != nil {
		return fmt.Errorf("msync failed: %w", err)
	}

	return nil
}

// Close unmaps and closes the device
func (d *Device) Close() error {
	if err := unix.Munmap(d.mmapData); err != nil {
		return err
	}
	return d.file.Close()
}
