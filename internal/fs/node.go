package fs

import (
	"os"
	"time"

	"bazil.org/fuse/fs"
)

// Node represents a filesystem node (file or directory)
type Node interface {
	fs.Node
}

// nodeAttr contains common attributes for files and directories
type nodeAttr struct {
	fs      *Filesystem // Reference to the filesystem
	inode   uint64      // Inode number
	name    string      // Name of the file/directory
	mode    os.FileMode // File mode/permissions
	uid     uint32      // User ID
	gid     uint32      // Group ID
	size    int64       // Size in bytes
	modTime time.Time   // Last modification time
}
