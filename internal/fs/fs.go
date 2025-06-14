package fs

import (
    // "context"
    "log"
    "os"
    "sync"
    "time"
    
    "bazil.org/fuse"
    "bazil.org/fuse/fs"
    "aethelfs/internal/dax"
)

// Filesystem implements a FUSE filesystem backed by a DAX device
type Filesystem struct {
    device       *dax.Device
    rootDir      *Dir
    inodeCount   uint64
    nextOffset   int64  // Track the next free offset
    offsetMutex  sync.Mutex  // Protect offset allocation
}

// NewFilesystem creates a new filesystem with the given DAX device
func NewFilesystem(device *dax.Device) (*Filesystem, error) {
    // Create a basic filesystem structure in the DAX device if it doesn't exist
    fs := &Filesystem{
        device:     device,
        inodeCount: 1, // Start with root inode
    }
    
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

// Node represents a filesystem node (file or directory)
type Node interface {
    fs.Node
}

// Common attributes for both files and directories
type nodeAttr struct {
    fs       *Filesystem
    inode    uint64
    name     string
    mode     os.FileMode
    uid, gid uint32
    size     uint64
    modTime  time.Time
}

// Fsync flushes all in-memory data to the DAX device
func (f *Filesystem) Fsync() error {
    // Use the DAX device's flush mechanism (msync or CLWB)
    return f.device.Flush()
}

// nextInode generates a new inode number
func (f *Filesystem) nextInode() uint64 {
    f.inodeCount++
    return f.inodeCount
}

// allocateSpace allocates space in the DAX device and returns the offset
func (f *Filesystem) allocateSpace(size int64) int64 {
    f.offsetMutex.Lock()
    defer f.offsetMutex.Unlock()
    
    // Initialize nextOffset if not already done
    if f.nextOffset == 0 {
        // We'll reserve the first 1MB for filesystem metadata
        f.nextOffset = 1024 * 1024
    }
    
    // Save the current offset
    offset := f.nextOffset
    
    // Advance to the next position with proper alignment
    alignment := int64(4096) // 4KB alignment
    newOffset := offset + size
    if newOffset % alignment != 0 {
        newOffset = ((newOffset / alignment) + 1) * alignment
    }
    f.nextOffset = newOffset
    
    // Check if we've exceeded device size
    daxData := f.device.MmapData()
    if offset + size > int64(len(daxData)) {
        // In production, you'd handle this better
        log.Printf("WARNING: Exceeded device size! Wrapping to beginning")
        f.nextOffset = 1024 * 1024 // Reset to start of data area
        return f.allocateSpace(size) // Try again
    }
    
    return offset
}

// Serve serves the filesystem over FUSE
func Serve(c *fuse.Conn, filesystem *Filesystem) error {
    return fs.Serve(c, filesystem)
}

// 
func (fs *Filesystem) CreateFile(name string) (*File, error) {
    // Get the DAX memory
    daxMemory := fs.device.MmapData()
    
    // Allocate space in your DAX memory tracking
    offset := fs.allocateSpace(1024) // Start with 1KB space
    
    // Create a slice view into the DAX memory
    fileData := daxMemory[offset:offset+1024]
    
    file := &File{
        nodeAttr: nodeAttr{
            name:    name,
            mode:    0644,
            uid:     uint32(os.Getuid()),
            gid:     uint32(os.Getgid()),
            modTime: time.Now(),
        },
        data:   fileData,
        fs:     fs,
        offset: offset,
        size:   0,
    }
    return file, nil
}
