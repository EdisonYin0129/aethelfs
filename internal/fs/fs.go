package fs

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"aethelfs/internal/common"
	"aethelfs/internal/dax"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Filesystem implements a FUSE filesystem backed by a DAX device
type Filesystem struct {
	device     *dax.Device
	rootDir    *Dir
	inodeCount uint64
	nextOffset int64      // Track the next free offset
	offsetMu   sync.Mutex // Protect offset allocation

	// Simple free space tracking
	freeSpaces   []freeSpace
	freeSpacesMu sync.Mutex
}

// Simple free space tracking structure
type freeSpace struct {
	offset int64
	size   int64
}

// NewFilesystem creates a new filesystem with the given DAX device
func NewFilesystem(device *dax.Device) (*Filesystem, error) {
	// Get total DAX device size
	daxSize := int64(len(device.MmapData()))

	// Create filesystem
	fs := &Filesystem{
		device:     device,
		inodeCount: 1, // Start with root inode
		// Reserve space for metadata
		nextOffset: common.MetadataReservationSize,
		// Initialize empty free space tracking
		freeSpaces: make([]freeSpace, 0),
	}

	// Log available space
	log.Printf("Filesystem initialized with %d MB available space",
		(daxSize-fs.nextOffset)/(1024*1024))

	// Create the root directory
	fs.rootDir = &Dir{
		nodeAttr: nodeAttr{
			fs:      fs,
			inode:   1,
			name:    "/",
			mode:    0755 | os.ModeDir,
			uid:     uint32(os.Getuid()),
			gid:     uint32(os.Getgid()),
			size:    4096,
			modTime: time.Now(),
		},
		children: make(map[string]Node),
	}

	return fs, nil
}

// Root implements the fs.FS interface and returns the root directory
func (f *Filesystem) Root() (fs.Node, error) {
	return f.rootDir, nil
}

// allocateSpace allocates space on the DAX device
func (f *Filesystem) allocateSpace(size int64) int64 {
	f.offsetMu.Lock()
	defer f.offsetMu.Unlock()

	// Round up size to alignment boundary
	alignedSize := ((size + common.BlockAlignmentSize - 1) /
		common.BlockAlignmentSize) * common.BlockAlignmentSize

	// First try to find space in the free list
	f.freeSpacesMu.Lock()
	defer f.freeSpacesMu.Unlock()

	for i, space := range f.freeSpaces {
		if space.size >= alignedSize {
			// Found suitable space
			offset := space.offset

			// Update or remove from free list
			if space.size > alignedSize {
				// Shrink the free space
				f.freeSpaces[i].offset += alignedSize
				f.freeSpaces[i].size -= alignedSize
			} else {
				// Remove this free space
				f.freeSpaces = append(f.freeSpaces[:i], f.freeSpaces[i+1:]...)
			}

			return offset
		}
	}

	// No suitable free space, allocate at the end
	offset := f.nextOffset

	// Update next available offset
	f.nextOffset += alignedSize

	return offset
}

// freeSpace returns space to the pool
func (f *Filesystem) freeSpace(offset int64, size int64) {
	if size <= 0 {
		return // Nothing to free
	}

	// Round up size to alignment boundary
	alignedSize := ((size + common.BlockAlignmentSize - 1) / common.BlockAlignmentSize) * common.BlockAlignmentSize

	f.freeSpacesMu.Lock()
	defer f.freeSpacesMu.Unlock()

	// Add to free list
	f.freeSpaces = append(f.freeSpaces, freeSpace{
		offset: offset,
		size:   alignedSize,
	})
}

// Fsync flushes filesystem changes to the DAX device
func (f *Filesystem) Fsync() error {
	// Check if device is available
	if f.device == nil {
		return fmt.Errorf("device not available")
	}

	// Try to flush, but handle potential errors
	err := f.device.Flush()
	if err != nil {
		// Log the error and continue
		fmt.Printf("Warning: device flush error: %v\n", err)
	}

	return nil
}

// nextInode generates a new inode number
func (f *Filesystem) nextInode() uint64 {
	f.inodeCount++
	return f.inodeCount
}

// CreateFile creates a new file with the given name
func (f *Filesystem) CreateFile(name string) (*File, error) {
	// Use default initial file size from constants
	initialSize := common.DefaultInitialFileSize

	// Allocate space for the file
	offset := f.allocateSpace(initialSize)

	// Get the data from the DAX device
	daxData := f.device.MmapData()

	// Create a new file object with the DAX slice
	file := &File{
		nodeAttr: nodeAttr{
			fs:      f,
			inode:   f.nextInode(),
			name:    name,
			mode:    0644,
			uid:     uint32(os.Getuid()),
			gid:     uint32(os.Getgid()),
			size:    0, // Initially empty
			modTime: time.Now(),
		},
		data:   daxData[offset : offset+initialSize],
		offset: offset,
		size:   0,
	}

	return file, nil
}

// Statfs implements the fs.FS interface and provides filesystem statistics
func (f *Filesystem) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	// Get total device size
	daxData := f.device.MmapData()
	totalSize := uint64(len(daxData))

	// Calculate used space with bounds checking
	// nextOffset tracks space allocation
	usedOffset := uint64(f.nextOffset)
	metadataReservation := uint64(common.MetadataReservationSize)

	// Ensure usedOffset is at least as large as the metadata reservation
	// to prevent underflow in the next calculation
	if usedOffset < metadataReservation {
		usedOffset = metadataReservation
	}

	// Calculate used space (nextOffset minus metadata reservation)
	usedSpace := usedOffset - metadataReservation

	// Calculate free space with bounds checking
	var freeSpace uint64
	if totalSize > usedSpace {
		freeSpace = totalSize - usedSpace
	} else {
		// If we somehow used more than the total (should not happen),
		// report zero free space
		freeSpace = 0
	}

	// Set a reasonable block size that aligns with most filesystem expectations
	blockSize := uint32(4096)

	// Calculate blocks with proper rounding up
	totalBlocks := (totalSize + uint64(blockSize) - 1) / uint64(blockSize)
	freeBlocks := (freeSpace + uint64(blockSize) - 1) / uint64(blockSize)

	// Sanity check to prevent reporting more free blocks than total
	if freeBlocks > totalBlocks {
		freeBlocks = totalBlocks
	}

	// Fill in the response
	resp.Blocks = totalBlocks         // Total data blocks
	resp.Bfree = freeBlocks           // Free blocks
	resp.Bavail = freeBlocks          // Available blocks (same as free for now)
	resp.Files = uint64(f.inodeCount) // Total files (inodes)
	resp.Ffree = uint64(1<<63 - 1)    // Free files (practically unlimited)
	resp.Bsize = blockSize            // Block size
	resp.Namelen = 255                // Maximum name length
	resp.Frsize = blockSize           // Fragment size (same as block size)

	// Log filesystem statistics if debug mode is enabled
	if *debugMode {
		fmt.Printf("Filesystem stats: total=%d MB, free=%d MB, used=%d MB (%.1f%%)\n",
			totalSize/(1024*1024),
			freeSpace/(1024*1024),
			usedSpace/(1024*1024),
			float64(usedSpace)*100.0/float64(totalSize))
	}

	return nil
}

// Serve serves the filesystem over FUSE
func Serve(c *fuse.Conn, filesystem *Filesystem) error {
	return fs.Serve(c, filesystem)
}
