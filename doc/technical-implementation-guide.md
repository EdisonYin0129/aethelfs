
### AethelFS Technical Implementation Guide

#### 1. Document Purpose

This document provides a detailed technical specification and phased implementation plan for AethelFS. It is intended for software engineers responsible for developing the filesystem and its associated management utilities. The document covers core concepts, on-disk data structures, and provides code examples for key implementation tasks.

#### 2. Language Choice (Go vs. Rust vs. C/C++)

While filesystems are traditionally written in C or C++ for maximum performance, and Rust is an excellent modern alternative, **Go is the recommended language for AethelFS**. This is a deliberate architectural decision based on the following trade-offs:

-   **C/C++ (The Legacy Choice)**
    
    -   **Pros:** Unmatched, "to-the-metal" performance with no runtime or Garbage Collector (GC) overhead. Direct, precise memory layout is native to the language.
        
    -   **Cons:** Manual memory safety is the primary drawback. Errors like use-after-free or buffer overflows are common and can lead to catastrophic data corruption. Concurrency via manual thread and lock management is notoriously difficult.
        
-   **Rust (The Performance & Safety Choice)**
    
    -   **Pros:** Performance is on par with C/C++. The compiler's ownership model and borrow checker _prove_ memory safety at compile time, eliminating memory bugs without a GC. FUSE support is excellent via crates like `fuser`.
        
    -   **Cons:** The learning curve is very steep, especially the ownership and lifetime concepts. While powerful, its concurrency models can be more complex to master than Go's for developers new to the language.
        
-   **Go (The Productivity & Safety Choice)**
    
    -   **Pros:** Memory safety is guaranteed by the runtime and garbage collector. Its concurrency model, using goroutines and channels, is famously simple and easy to reason about, which is a perfect fit for a FUSE daemon. The development cycle is extremely fast.
        
    -   **Cons:** The runtime and GC introduce a small, though highly optimized, performance overhead compared to Rust or C++.
        

**Conclusion:** For AethelFS, the enormous gains in **developer productivity, safety, and simplicity of concurrency** provided by Go far outweigh the marginal performance trade-off. The primary goal for the MVP is a _robust and correct_ filesystem built within a reasonable timeframe, making Go the most strategic choice for this project.

#### 3. Project Directory Structure

To ensure a clean and scalable codebase, the project should adhere to the following directory structure, which is standard for Go projects:

```
aethelfs/
├── cmd/
│   ├── apool/
│   │   └── main.go        // CLI for pool management
│   ├── afs/
│   │   └── main.go        // CLI for filesystem management
│   └── aethelfsd/
│       └── main.go        // The FUSE daemon executable
├── internal/
│   ├── disk/
│   │   └── layout.go      // Go structs for on-disk data structures
│   ├── pool/
│   │   └── manager.go     // Logic for apool (create, list, destroy)
│   └── fs/
│       ├── fs.go          // FUSE server boilerplate and main FS struct
│       ├── dir.go         // Implementation for directory operations
│       ├── file.go        // Implementation for file operations
│       └── superblock.go  // Logic for managing the superblock/uberblock
├── scripts/
│   └── setup_test_env.sh  // Helper scripts for creating backing files, etc.
├── go.mod
└── go.sum

```

#### 4. Core Technologies

A successful implementation requires a functional understanding of the following system-level technologies.

-   **Filesystem Principles:** The implementation will create POSIX-like filesystem structures, including inodes, data blocks, directories, and a superblock.
    
    -   **Inode:** Metadata container for file attributes (mode, size, timestamps, etc.) and an array of block pointers.
        
    -   **Directory:** A data block containing a series of `(filename, inode_number)` records.
        
    -   **Superblock:** A filesystem-wide structure containing pointers to root metadata objects (e.g., inode table, block allocation bitmap). In AethelFS, the function of a traditional superblock is fulfilled by the **Uberblock** mechanism.
        
-   **FUSE (Filesystem in Userspace):** A kernel module and protocol that proxies VFS (Virtual File System) calls to a user-space daemon. This avoids kernel module development. All filesystem logic (read, write, lookup, etc.) will be handled by the `aethelfsd` daemon.
    
-   **DAX (Direct Access):** A kernel mechanism for memory-like devices (e.g., CXL memory expanders) that exposes the device as `/dev/daxX.Y`. DAX allows the `mmap()` system call to map the device's physical address range directly into an application's virtual address space, bypassing the kernel page cache for minimal latency.
    
-   **`mmap(2)` / `msync(2)` System Calls:**
    
    -   `mmap(2)`: The primary interface to the backing device. The entire device will be mapped into the `aethelfsd` daemon's address space as a byte slice (`[]byte`). All filesystem structures will be treated as objects within this slice.
        
    -   `msync(2)`: Guarantees persistence. After a metadata update, `msync()` with the `MS_SYNC` flag will be called to ensure data is flushed from CPU caches to the durable medium. This is critical for crash consistency.
        

#### 5. Phased Implementation Plan

Development is divided into three sequential phases. Each phase builds upon the last, culminating in a functional filesystem.

##### Phase 1: Pool Management & On-Disk Structures (`apool`)

**Objective:** Implement the `apool` utility to manage the physical lifecycle of storage pools. This phase focuses on correctly defining and writing the fundamental on-disk data structures.

-   **Task 1.1: Define On-Disk Go Structs**
    
    -   Create a `disk` package (`internal/disk/layout.go`). Define Go structs with fixed-size fields that directly correspond to the binary layout on disk. Use the `encoding/binary` package for serialization and deserialization.
        
    
    ```
    package disk
    
    import "unsafe"
    
    const (
        LabelSize      = 256 * 1024
        UberblockMagic = 0xA37BE1F5
        NVListMaxLen   = 112 * 1024
        // ... other constants
    )
    
    // Uberblock is the 1KB structure pointing to the root of the FS.
    // There are 128 of these in each Label.
    type Uberblock struct {
        Magic     uint64
        Version   uint64
        TXG       uint64 // Transaction Group ID
        GUIDSum   uint64
        Timestamp uint64
        RootBP    uint64 // Block Pointer to Superblock
        // Padding to 1KB
        _ [968]byte
    }
    
    // Label is the 256KB structure at the start/end of a device.
    type Label struct {
        _           [16 * 1024]byte // Blank space
        NVList      [NVListMaxLen]byte
        Uberblocks  [128]Uberblock
    }
    
    ```
    
-   **Task 1.2: Implement `apool create`**
    
    -   The `create` command must correctly provision a file or device and write the four redundant labels.
        
    
    ```
    // Example logic snippet for writing a label
    func writeLabel(device *os.File, label *disk.Label, offset int64) error {
        if _, err := device.Seek(offset, io.SeekStart); err != nil {
            return fmt.Errorf("failed to seek to offset %d: %w", offset, err)
        }
    
        // binary.Write is used to serialize the Go struct into its binary representation
        // directly to the file descriptor.
        if err := binary.Write(device, binary.LittleEndian, label); err != nil {
            return fmt.Errorf("failed to write label at offset %d: %w", offset, err)
        }
        return nil
    }
    
    // In the main command function:
    // 1. OpenFile(devicePath, os.O_RDWR, 0)
    // 2. If it's a file, file.Truncate(size)
    // 3. Create a label in memory: label := &disk.Label{...}
    // 4. Populate NVList (e.g., with JSON marshaled data)
    // 5. writeLabel(device, label, 0)
    // 6. writeLabel(device, label, disk.LabelSize)
    // 7. writeLabel(device, label, size - (2 * disk.LabelSize))
    // 8. writeLabel(device, label, size - disk.LabelSize)
    
    ```
    
-   **Task 1.3: Implement `apool list`**
    
    -   Scan candidate devices, read the first label, and validate it by checking for the `UberblockMagic`. Parse the `NVList` to display pool info.
        
-   **Task 1.4: Implement `apool destroy`**
    
    -   Open the device and write zeros over the first few kilobytes of each of the four label areas to invalidate them.
        

##### Phase 2: Filesystem Initialization (`afs`)

**Objective:** Implement the `afs create` command to lay down the initial filesystem metadata within a pool.

-   **Task 2.1: Define Core Filesystem Structs**
    
    -   In the `disk` package, add structs for `Superblock`, `Inode`, and `DirectoryEntry`.
        
-   **Task 2.2: Implement `afs create`**
    
    -   This command orchestrates the transition from an empty pool to a mountable filesystem.
        
    
    ```
    // In the afs create command:
    // 1. Find the pool's backing device.
    // 2. Memory-map the file/device using a library like golang.org/x/exp/mmap.
    //    mmapFile, err := mmap.Open(devicePath)
    //    defer mmapFile.Close()
    //
    // 3. Define the layout within the "Usable Data Area" (the space between labels).
    //    superblockOffset := uint64(disk.LabelSize * 2)
    //    inodeBitmapOffset := superblockOffset + disk.BlockSize
    //    // ... and so on for other metadata areas.
    //
    // 4. Get pointers/slices to these areas from the mapped region.
    //    // This is a simplified representation. You'll need functions to safely
    //    // access and cast parts of the mmap'd byte slice.
    //    superblock := (*disk.Superblock)(unsafe.Pointer(&mappedData[superblockOffset]))
    //    inodeBitmap := mappedData[inodeBitmapOffset : ...]
    //
    // 5. Initialize the metadata: zero out bitmaps, create the root inode in the inode table.
    //
    // 6. Create the first Uberblock, pointing its RootBP to the Superblock's block address.
    //
    // 7. Write this new Uberblock to all four labels and call mmapFile.Flush()
    //    (which corresponds to msync).
    
    ```
    

##### Phase 3: The FUSE Daemon (`aethelfsd`)

**Objective:** Implement the filesystem daemon to serve file I/O requests from the kernel.

-   **Task 3.1: FUSE Server Boilerplate**
    
    -   Use a library like `bazil.org/fuse` to handle the low-level FUSE protocol.
        
    
    ```
    // cmd/aethelfsd/main.go
    package main
    
    import (
        "flag"
        "log"
        "os"
    
        "bazil.org/fuse"
        "bazil.org/fuse/fs"
        "aethelfs/internal/fs" // Your filesystem implementation
    )
    
    func main() {
        // ... argument parsing for mountpoint ...
        mountpoint := flag.Arg(0)
    
        c, err := fuse.Mount(
            mountpoint,
            fuse.FSName("aethelfs"),
            fuse.Subtype("aethelfs"),
        )
        if err != nil { log.Fatal(err) }
        defer c.Close()
    
        // Create your FS struct after finding and mmap'ing the device
        filesys := fs.NewFS(...) // This constructor handles mount logic
    
        if err := fs.Serve(c, filesys); err != nil {
            log.Fatal(err)
        }
    }
    
    ```
    
-   **Task 3.2: Mount Logic**
    
    -   Before serving, the daemon must find the most recent valid state by scanning all 512 uberblocks (128 per label) and selecting the one with the highest valid `TXG`. The `RootBP` from this uberblock is the entry point to the live filesystem.
        
-   **Task 3.3 & 3.4: Implement FUSE Operations**
    
    -   Implement the `fs.Node` and related interfaces (`fs.Handle`, `fs.NodeStringLookuper`, etc.) for your `File` and `Dir` types in the `internal/fs` package.
        
    
    ```
    package fs // In internal/fs/dir.go
    
    // Dir represents a directory node in the filesystem.
    type Dir struct{
        inodeNum uint64
        fs *FS // Pointer back to the main FS object
    }
    
    // Attr sets the attributes for the directory.
    func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
        // 1. Find the inode struct in the mmap'd inode table using d.inodeNum
        // 2. Copy the attributes from your on-disk inode struct to the fuse.Attr struct.
        inode := d.fs.getInode(d.inodeNum)
        a.Inode = d.inodeNum
        a.Mode = os.ModeDir | os.FileMode(inode.Permissions)
        a.Size = inode.Size
        // ... set other attributes
        return nil
    }
    
    // Lookup looks up a specific entry in the directory.
    func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
        // 1. Find the inode for this directory 'd'.
        // 2. Iterate through the data blocks pointed to by the inode.
        // 3. Scan the DirectoryEntry records in those blocks.
        // 4. If an entry with the matching 'name' is found, get its inode number.
        // 5. Check that entry's inode to see if it's a file or directory.
        // 6. Return a new &Dir{} or &File{} node for it.
        // 7. If not found, return fuse.ENOENT.
        return nil, fuse.ENOENT
    }
    
    ```
    

#### 6. Development & Debugging Notes

-   **File-Based Pools:** All initial development must be done with file-backed pools for ease of inspection and iteration.
    
-   **Hex Editor:** Use of a hex editor (`xxd`, `hexdump`) is mandatory for verifying the on-disk layout of structs and ensuring correct byte-level manipulation.
    
-   **Foreground FUSE:** Run the daemon in the foreground with debug output enabled for live diagnostics: `go run ./cmd/aethelfsd -d <pool> <mountpoint>`. The `-d` flag in `bazil.org/fuse` enables verbose protocol-level logging.
    
-   **Source Control:** Use Git. Commit atomically after each functional change. A well-structured commit history is crucial for debugging regressions.
