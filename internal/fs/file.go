package fs

import (
    "context"
    "fmt"
    "time"
    // "unsafe"
    
    "bazil.org/fuse"
    // "aethelfs/pkg/cache"
)

// File represents a file in the filesystem
type File struct {
    nodeAttr
    data []byte  // This should be a slice of the mmap'd region
    fs   *Filesystem
    offset int64 // Position in the DAX memory
    size   int64 // Size of this file
}

// Attr implements the fs.Node interface
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
    a.Inode = f.inode
    a.Mode = f.mode
    a.Uid = f.uid
    a.Gid = f.gid
    a.Size = uint64(f.size)
    a.Mtime = f.modTime
    a.Ctime = f.modTime
    a.Atime = f.modTime
    return nil
}

// ReadAll implements the fs.HandleReadAller interface
func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
    fmt.Printf("ReadAll called for file: %s (size: %d)\n", f.name, f.size)
    // Only return the valid portion of the data up to the file's logical size
    if f.size <= 0 {
        return []byte{}, nil
    }
    
    // Make sure we don't go out of bounds
    if f.size > int64(len(f.data)) {
        f.size = int64(len(f.data))
    }
    
    // Return only the valid data
    return f.data[:f.size], nil
}

// Read implements the fs.HandleReader interface
func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
    if req.Offset >= f.size {
        // Reading past EOF
        resp.Data = make([]byte, 0)
        return nil
    }
    
    // Calculate how much data to read
    end := req.Offset + int64(req.Size)
    if end > f.size {
        end = f.size
    }
    
    // Make a copy of the data - use standard slice copy which is safer
    length := end - req.Offset
    resp.Data = make([]byte, length)
    copy(resp.Data, f.data[req.Offset:end])
    
    return nil
}

// Helper function to find minimum of two integers
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// Write implements the fs.HandleWriter interface
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
    // If we need to extend the file, we must reallocate in DAX memory
    newSize := req.Offset + int64(len(req.Data))
    
    fmt.Printf("Write request: file=%s, offset=%d, dataLen=%d, newSize=%d\n", 
               f.name, req.Offset, len(req.Data), newSize)
    
    if newSize > int64(len(f.data)) {
        // Get a new, larger slice from DAX memory
        daxMemory := f.fs.device.MmapData()
        
        // Allocate new space - use a unique offset for each file!
        newOffset := f.fs.allocateSpace(newSize)
        
        // Create a new slice from DAX memory
        newData := daxMemory[newOffset:newOffset+newSize]
        
        // Copy existing data
        copy(newData, f.data[:f.size])
        
        // Update file with new DAX slice
        f.data = newData
        f.offset = newOffset
    }
    
    // Write the data directly to the DAX-mapped region
    copy(f.data[req.Offset:], req.Data)
    
    // Update size and time
    if newSize > f.size {
        f.size = newSize
    }
    f.modTime = time.Now()
    resp.Size = len(req.Data)
    
    // Flush changes to ensure persistence
    f.fs.Fsync()
    
    fmt.Printf("Write success: wrote %d bytes, new size=%d\n", len(req.Data), f.size)
    return nil
}

// Flush implements the fs.HandleFlusher interface
func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
    return f.fs.Fsync()
}

// Fsync implements the fs.HandleFsyncer interface
func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
    return f.fs.Fsync()
}

// Setattr implements the fs.NodeSetattrer interface
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
    if req.Valid.Size() {
        // Truncate the file
        newSize := int64(req.Size)
        
        // If we need to extend the file
        if newSize > int64(len(f.data)) {
            // Get a new, larger slice from DAX memory
            daxMemory := f.fs.device.MmapData()
            
            // Allocate new space
            newOffset := f.fs.allocateSpace(newSize)
            
            // Create a new slice from DAX memory
            newData := daxMemory[newOffset:newOffset+newSize]
            
            // Copy existing data
            copy(newData, f.data[:f.size])
            
            // Update file with new DAX slice
            f.data = newData
            f.offset = newOffset
        }
        
        // Update the size
        f.size = newSize
    }
    
    // Update other attributes
    if req.Valid.Mode() {
        f.mode = req.Mode
    }
    if req.Valid.Uid() {
        f.uid = req.Uid
    }
    if req.Valid.Gid() {
        f.gid = req.Gid
    }
    if req.Valid.Mtime() {
        f.modTime = req.Mtime
    }
    
    return nil
}

// Release implements the fs.HandleReleaser interface
func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
    fmt.Printf("Release called for file: %s\n", f.name)
    // Don't return any errors here - this is critical!
    return nil
}
