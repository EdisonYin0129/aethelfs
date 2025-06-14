package dax

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	// "time"
)

// Device represents a DAX character device (/dev/daxX.Y)
type Device struct {
	file    *os.File
	size    int64
	mmapData []byte
}

// NewDevice opens a DAX device and maps it into memory
func NewDevice(path string) (*Device, error) {
	// Check if the path is a DAX device
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
	
	// For DAX character devices, stat.Size() is often 0
	if size <= 4096 {
		fmt.Printf("DAX device %s reported size 0, using fixed size instead\n", path)
		// Use a much smaller initial size
		size = 16 * 1024 * 1024 // Start with just 16MB
	}
	
	// Respect the device's alignment requirements
	alignmentSize := int64(2 * 1024 * 1024) // 2MB alignment
	if size % alignmentSize != 0 {
		// Round up to the next alignment boundary
		size = ((size / alignmentSize) + 1) * alignmentSize
	}
	
	fmt.Printf("Attempting to mmap DAX device with size: %d bytes\n", size)
	
	// Direct mmap approach - skip the goroutine for now
	mmapData, err := unix.Mmap(int(file.Fd()), 0, int(size), 
	                           unix.PROT_READ|unix.PROT_WRITE, 
	                           unix.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap DAX device: %v", err)
	}
	
	fmt.Printf("Successfully mapped DAX device, address: %p, len: %d\n", 
	           mmapData, len(mmapData))
	
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

// MmapData returns the memory-mapped data for the DAX device
func (d *Device) MmapData() []byte {
	return d.mmapData
}

// Flush ensures data is persisted to the device
// This can use msync (slow) or CLWB (fast) if available
func (d *Device) Flush() error {
	return unix.Msync(d.mmapData, unix.MS_SYNC)
}

// Close unmaps the device and closes the file
func (d *Device) Close() error {
	if err := unix.Munmap(d.mmapData); err != nil {
		return err
	}
	return d.file.Close()
}