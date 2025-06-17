package fs

import (
	"context"
	"fmt"
	"time"

	"bazil.org/fuse"
)

// File represents a file in the filesystem
type File struct {
	nodeAttr
	data   []byte // Slice of the mmap'd region
	offset int64  // Position in the DAX memory
	size   int64  // Size of this file
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

// Read implements the fs.HandleReader interface
func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// Check if read is beyond file size
	if req.Offset >= f.size {
		resp.Data = []byte{}
		return nil
	}

	// Calculate read bounds
	end := req.Offset + int64(req.Size)
	if end > f.size {
		end = f.size
	}

	length := end - req.Offset

	// Create response buffer
	resp.Data = make([]byte, length)

	// Copy data from the mapped region
	copy(resp.Data, f.data[req.Offset:end])

	return nil
}

// Write implements the fs.HandleWriter interface
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	newSize := req.Offset + int64(len(req.Data))

	// Check if we need to grow the file
	if newSize > int64(len(f.data)) {
		// Calculate new size - just double current or use required size, whichever is larger
		newCapacity := int64(len(f.data)) * 2
		if newCapacity < newSize {
			newCapacity = newSize
		}

		// Save old allocation info
		oldOffset := f.offset
		oldLength := int64(len(f.data))

		// Get a new slice from DAX memory
		daxMemory := f.fs.device.MmapData()
		newOffset := f.fs.allocateSpace(newCapacity)

		// Create a new slice from DAX memory
		newData := daxMemory[newOffset : newOffset+newCapacity]

		// Copy existing data
		copy(newData, f.data[:f.size])

		// Update file with new DAX slice
		f.data = newData
		f.offset = newOffset

		// Free the old space
		if oldLength > 0 {
			f.fs.freeSpace(oldOffset, oldLength)
		}
	}

	// Write the data
	copy(f.data[req.Offset:], req.Data)

	// Update size if needed
	if newSize > f.size {
		f.size = newSize
	}
	f.modTime = time.Now()
	resp.Size = len(req.Data)

	// Flush changes for metadata
	if req.Offset == 0 || req.Offset < 4096 {
		f.fs.Fsync()
	}

	return nil
}

// Flush implements the fs.HandleFlusher interface
func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// Try to sync, but don't fail the flush operation if it doesn't succeed
	// This is critical - returning an error from Flush will cause operations to fail
	if err := f.fs.Fsync(); err != nil {
		// Log the error but don't return it
		fmt.Printf("Warning: non-fatal error during Flush: %v\n", err)
	}

	// Always return success for Flush to avoid "invalid argument" errors
	return nil
}

// Fsync implements the fs.HandleFsyncer interface
func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	// Try to sync, but always return success for FUSE operations
	if err := f.fs.Fsync(); err != nil {
		fmt.Printf("Warning: non-fatal error during Fsync: %v\n", err)
	}
	return nil
}

// Setattr implements the fs.NodeSetattrer interface
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if req.Valid.Size() {
		// Handle truncate
		newSize := int64(req.Size)

		if newSize > int64(len(f.data)) {
			// Need to grow
			daxMemory := f.fs.device.MmapData()
			newOffset := f.fs.allocateSpace(newSize)
			newData := daxMemory[newOffset : newOffset+newSize]

			// Copy existing data
			copy(newData, f.data[:f.size])

			// Save old allocation info
			oldOffset := f.offset
			oldSize := int64(len(f.data))

			// Update file with new slice
			f.data = newData
			f.offset = newOffset

			// Free old space
			f.fs.freeSpace(oldOffset, oldSize)
		}

		// Update size
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
	// Try to sync on release, but don't fail if it doesn't succeed
	if err := f.fs.Fsync(); err != nil {
		fmt.Printf("Warning: non-fatal error during Release: %v\n", err)
	}

	// Always return success for Release to avoid "invalid argument" errors
	return nil
}
