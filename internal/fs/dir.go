package fs

import (
    "context"
    "os"
    "syscall"
    "time"

    "bazil.org/fuse"
    "bazil.org/fuse/fs"
)

// Dir represents a directory in the filesystem
type Dir struct {
    nodeAttr
    children map[string]Node
}

// Attr implements the fs.Node interface
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
    a.Inode = d.inode
    a.Mode = d.mode
    a.Uid = d.uid
    a.Gid = d.gid
    a.Size = uint64(d.size)
    a.Mtime = d.modTime
    a.Ctime = d.modTime
    a.Atime = d.modTime
    return nil
}

// Lookup implements the fs.NodeStringLookuper interface
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
    if child, ok := d.children[name]; ok {
        return child, nil
    }
    return nil, syscall.ENOENT
}

// ReadDirAll implements the fs.HandleReadDirAller interface
func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
    var dirents []fuse.Dirent
    for name, node := range d.children {
        // Determine the type of the node
        var typ fuse.DirentType
        if _, ok := node.(*File); ok {
            typ = fuse.DT_File
        } else if _, ok := node.(*Dir); ok {
            typ = fuse.DT_Dir
        }

        // Get the inode number
        var attr fuse.Attr
        node.(fs.Node).Attr(ctx, &attr)
        
        dirents = append(dirents, fuse.Dirent{
            Inode: attr.Inode,
            Type:  typ,
            Name:  name,
        })
    }
    return dirents, nil
}

// Mkdir implements the fs.NodeMkdirer interface
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
    child := &Dir{
        nodeAttr: nodeAttr{
            fs:      d.fs,
            inode:   d.fs.nextInode(),
            name:    req.Name,
            mode:    req.Mode | os.ModeDir,
            uid:     req.Uid,
            gid:     req.Gid,
            size:    4096,
            modTime: time.Now(),
        },
        children: make(map[string]Node),
    }

    d.children[req.Name] = child
    d.modTime = time.Now()
    d.fs.Fsync() // Flush changes to the DAX device

    return child, nil
}

// Create implements the fs.NodeCreater interface
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
    // Create a new file using the filesystem's CreateFile method
    child, err := d.fs.CreateFile(req.Name)
    if err != nil {
        return nil, nil, err
    }
    
    // Update the child's attributes based on the request
    child.nodeAttr.inode = d.fs.nextInode()
    child.nodeAttr.mode = req.Mode
    child.nodeAttr.uid = req.Uid
    child.nodeAttr.gid = req.Gid
    child.nodeAttr.modTime = time.Now()
    
    // Add to directory entries
    d.children[req.Name] = child
    d.modTime = time.Now()
    d.fs.Fsync() // Flush changes
    
    return child, child, nil
}

// Remove implements the fs.NodeRemover interface
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
    if _, ok := d.children[req.Name]; !ok {
        return syscall.ENOENT
    }

    delete(d.children, req.Name)
    d.modTime = time.Now()
    d.fs.Fsync() // Flush changes to the DAX device

    return nil
}
