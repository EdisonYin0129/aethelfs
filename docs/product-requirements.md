### Product Requirements Document: AethelFS v1.0 (MVP)

#### 1. Introduction & Vision

**Product Name:** AethelFS (pronounced "Ethel-F-S")

**Vision:** To provide a simple, high-performance, user-space filesystem specifically designed for Compute Express Link (CXL) memory and other memory-like devices. AethelFS prioritizes ease of use and a familiar administrative experience, drawing inspiration from ZFS's powerful and intuitive command-line interface. It will provide a straightforward path for developers and administrators to leverage the speed of CXL memory without the complexity of kernel development.

**Name and Rationale:** The name "Aethel" is an Old English word for "noble," chosen to pay homage to the noble design heritage of ZFS. While several memory-based filesystems exist, many are designed for block devices and do not properly support the `mmap()` interface on DAX character devices, which is key to unlocking the lowest latency. Other specialized filesystems like NOVA (`NVSL/linux-nova`) are now largely defunct, while active projects like FAMFS (`cxl-micron-reskit/famfs`) focus on more complex use cases like shared memory across hosts. AethelFS fills a niche by providing a simple, modern, single-host filesystem built from the ground up for the DAX `mmap` model.

#### 2. Target Audience

-   **Linux System Administrators:** Responsible for provisioning and managing storage for applications that require extremely low latency (e.g., databases, caches, scientific computing).
    
-   **Developers:** Working on applications that can benefit from direct, low-latency access to persistent or volatile memory tiers. They need a simple way to test and develop without requiring physical CXL hardware.
    

#### 3. Guiding Principles

-   **Simplicity Over Complexity:** The internal design should be as simple as possible. We will not implement RAID or other complex volume management features, relying instead on lower-level system components (BIOS/Kernel MD) for device aggregation.
    
-   **Familiar User Experience:** The command-line tools will be intentionally modeled after ZFS's `zpool` and `zfs` commands for a gentle learning curve.
    
-   **Userspace First:** The entire filesystem will be implemented in userspace via FUSE, eliminating the need for custom kernel modules and ensuring broad compatibility and rapid development.
    
-   **Testability:** Support for file-backed device simulation is a first-class feature, enabling development and testing on any Linux machine.
    


#### 4. System Architecture

AethelFS consists of two main components: the management utilities and the FUSE daemon.

-   **Management Utilities (`apool`, `afs`):** These CLIs are responsible for all offline management tasks: creating and destroying pools, initializing filesystems within those pools, and launching the FUSE daemon to "mount" a filesystem.
    
-   **FUSE Daemon (`aethelfsd`):** This is the core filesystem process. It is launched by the `afs mount` command. It reads the AethelFS metadata from the backing device, manages space allocation, and translates VFS (Virtual File System) calls from the kernel into operations on the memory-mapped device.
    

**High-Level Architectural Diagram:**

```
+-------------------------------------------------------------------------+
|                                  User                                   |
+-------------------------------------------------------------------------+
      |                |                  |                  |
      | RUNS           | RUNS             | interacts w/     | RUNS
      |                |                  | mount point      |
+-----v----+      +----v-----+       +----v-------------+    |
|  `apool` |      |  `afs`   |       |   (bash, ls, cp) |    |
|   (CLI)  |      |  (CLI)   |       +------------------+    |
+----------+      +----------+                  |            |
      |                |                        | POSIX      |
      | Manages        | Manages & LAUNCHES     | API Calls  |
      |                |                        |            |
+-----v----------------v------------------------v------------v------------+
|                        Kernel Space (VFS)                               |
+-------------------------------------------------------------------------+
                                      |
                                      | FUSE Protocol
                                      v
+-------------------------------------------------------------------------+
|                          Userspace AethelFS                             |
|                                                                         |
|  +-------------------------------------------------------------------+  |
|  |                     AethelFS Daemon (`aethelfsd`)                 |  |
|  |                                                                   |  |
|  |  +-----------------+  +-----------------+  +-------------------+  |  |
|  |  | FUSE Request    |  | Space Allocator |  | Metadata Manager  |  |  |
|  |  | Handler         |==| (manages space) |==| (Inodes, Dirs)    |  |  |
|  |  | (read, write,..)|  |                 |  | (manages files)   |  |  |
|  |  +-----------------+  +-----------------+  +-------------------+  |  |
|  |                          |                                        |  |
|  |                          | mmap() / msync()                       |  |
|  |                          v                                        |  |
|  +-------------------------------------------------------------------+  |
+-------------------------------------------------------------------------+
                                      |
                                      | Read/Write
                                      v
+-------------------------------------------------------------------------+
|     Backing Device (Pool) - mmap'd into daemon's address space          |
|                                                                         |
|  /dev/daxX.Y (CXL Device) OR /path/to/backing_file.img                  |
+-------------------------------------------------------------------------+

```

#### 5. On-Device Data Structures & Metadata

To ensure robustness, AethelFS adopts a redundant labeling scheme inspired by ZFS. Each device in a pool contains four labels, two at the beginning and two at the end. This provides strong protection against accidental overwrites or localized media corruption.

**On-Device Layout:**

```
+------------------+------------------+----------------------------------+------------------+------------------+
|    Label 0       |    Label 1       |         Usable Data Area         |    Label 2       |    Label 3       |
|     256KB        |     256KB        |      (Filesystem data blocks,    |     256KB        |     256KB        |
|                  |                  |       bitmaps, inode tables)     |                  |                  |
+------------------+------------------+----------------------------------+------------------+------------------+
^                  ^                                                     ^                  ^
|                  |                                                     |                  |
0KB              256KB                                           (Size - 512KB)     (Size - 256KB)

```

**Label Structure (256KB):**

Each of the four labels is identical in structure and contains the full configuration for the pool.

```
+-----------------------------------------------------------------------------+
| Offset | Size   | Description                                                 |
+-----------------------------------------------------------------------------+
| 0KB    | 16KB   | Blank Space (Reserved for bootloaders, VTOC, etc.)          |
+--------+--------+-------------------------------------------------------------+
| 16KB   | 112KB  | NVList (Name-Value List) Area                               |
|        |        | - XDR encoded key-value pairs                               |
|        |        | - e.g., pool_name, pool_guid, version, features, hostname   |
|        |        | - state: (ACTIVE, EXPORTED, etc.)                           |
|        |        | - txg: transaction group number for this label write        |
+--------+--------+-------------------------------------------------------------+
| 128KB  | 128KB  | Uberblock Array (128 x 1KB structures)                      |
|        |        | - A circular log of pointers to the root of the filesystem. |
|        |        | - Each write creates a new Uberblock.                       |
|        |        | - The one with the highest valid transaction ID is active.  |
+-----------------------------------------------------------------------------+

```

**Uberblock Structure (1KB):**

The Uberblock is the bridge from the fixed-location label to the variable-location live filesystem data.

```
- magic:      0xA37BE1F5 (AethelFS Uberblock)
- version:    Filesystem version
- txg:        Transaction group number
- guid_sum:   Checksum of all device GUIDs in the pool
- timestamp:  Time of this transaction
- root_bp:    Block Pointer to the Filesystem Superblock

```

**Metadata Class Diagram (Updated):**

The daemon now finds the active Uberblock across all labels, which in turn points to the single live Superblock.

```
+-----------------+
| AethelFSDaemon  |
+-----------------+
        | 1
        | scans
        v 4
+-----------------+      contains      +-----------------+
|      Label      |------------------->| Uberblock Array |
+-----------------+   128 per label    +-----------------+
                                               | (finds active)
                                               | 1
                                               v
+-----------------+      manages       +----------------+
|   Superblock    |------------------->|  InodeBitmap   |
| (pointed to by  |                    +----------------+
| active Uberblock)|                    +----------------+
+-----------------+------------------->| DataBlockBitmap|
        | 1                            +----------------+
        | has-an
        v
+-----------------+
|   InodeTable    |
+-----------------+
        | 1..*
        | contains
        v
+-----------------+
|      Inode      |
+-----------------+

```

#### 6. Command Line Interface (CLI)

The CLI design remains the same, but the underlying actions are now more robust.

-   `apool create` will write all four labels, populating the NVList with pool information.
    
-   `afs create` will find the pool's labels and write the first Uberblock, pointing it to a newly allocated and initialized Superblock.
    
-   `afs mount` will scan all four labels on the device, find the valid Uberblock with the highest transaction group number (`txg`), and use its `root_bp` to find and `mmap` the filesystem

##### 6.1. Pool Management (`apool`)

The `apool` command manages the physical storage pools.

-   **`apool create [-s <size>] <poolname> <device|file>`**
    
    -   Creates a new pool named `<poolname>` on the specified device or file.
        
    -   `-s`: If creating on a file, specifies the size (e.g., `1G`, `256M`).
        
    -   Writes the **Pool Label** to Block 0 of the device.
        
-   **`apool list [-f <format>]`**
    
    -   Lists all discoverable AethelFS pools on the system by scanning devices.
        
    -   `-f`: Output format. `table` (default) or `json`.
        
-   **`apool destroy <poolname>`**
    
    -   Destroys a pool. Wipes the label from the device. A very dangerous operation that will require confirmation.
        

**Example `apool` Usage:**

```bash
# Create a 1GB file-backed pool named "testpool"
$ sudo apool create -s 1G testpool /tmp/aethelfs.img
Pool 'testpool' created successfully.

# Create a pool on a CXL device
$ sudo apool create tank /dev/dax0.0
Pool 'tank' created successfully.

# List pools (default table format)
$ apool list
NAME      DEVICE              SIZE    FILESYSTEM
testpool  /tmp/aethelfs.img   1.00G   <none>
tank      /dev/dax0.0         64.0G   tank

# List pools (JSON format)
$ apool list -f json
[
  {
    "name": "testpool",
    "device": "/tmp/aethelfs.img",
    "size": 1073741824,
    "filesystem": null
  },
  {
    "name": "tank",
    "device": "/dev/dax0.0",
    "size": 68719476736,
    "filesystem": "tank"
  }
]
```

##### 6.2. Filesystem Management (`afs`)

The `afs` command manages filesystems within a pool.

-   **`afs create <poolname>`**
    
    -   Creates a new AethelFS filesystem that uses the entirety of the specified pool.
        
    -   This writes the **Superblock** and initializes the bitmaps and root inode.
        
-   **`afs mount <poolname> <mountpoint>`**
    
    -   Mounts the filesystem from `<poolname>` at the specified `<mountpoint>`.
        
    -   This command starts the `aethelfsd` FUSE daemon in the background.
        
-   **`afs unmount <mountpoint>`**
    
    -   Unmounts the filesystem. Terminates the FUSE daemon.
        
-   **`afs list [-f <format>]`**
    
    -   Lists all created AethelFS filesystems and their status.
        
-   **`afs destroy <poolname>`**
    
    -   Destroys the filesystem within a pool by clearing the superblock.
        

#### 7. Man Pages

##### `apool(8)`


APOOL(1)                  User Commands                 APOOL(1)

NAME
    apool - AethelFS pool management utility

SYNOPSIS

    apool <subcommand> [arguments]

DESCRIPTION

The apool utility manages AethelFS storage pools. A storage pool is a single DAX device or file that serves as a backing store for an AethelFS filesystem. Each pool is identified by a unique name. This utility is used for creating, destroying, and listing available pools.

SUBCOMMANDS

    create [-f] [-s size] poolname device
        Creates a new storage pool with the given poolname on the specified device. The device can be a /dev/daxX.Y character device or a path to a regular file.

        -s size
            Required when device is a file. Specifies the size of the file to create. The size may be specified in bytes, or with a suffix (K, M, G, T).

        -f
            Force the creation of the pool, even if the device appears to contain data.

    destroy [-f] poolname
        Destroys the specified pool. This is a destructive operation that erases the AethelFS label from the backing device, making the data unrecoverable by AethelFS. The command will fail if a filesystem currently exists on the pool unless the force flag is used.

        -f
            Force the destruction of the pool, even if it contains an active filesystem.

    list [-H] [-o field,...] [-p format]
        Lists all visible AethelFS pools on the system.

        -H
            Do not print headers.

        -o field,...
            A comma-separated list of fields to display. Available fields are: name, device, size, fsname.

        -p format
            Print output in a specific format. Supported formats are table (default) and json.

EXAMPLES

    sudo apool create -s 2G mypool /data/aethelfs.img
        Creates a 2-gigabyte file-backed pool named "mypool".

    sudo apool create tank /dev/dax0.1
        Creates a pool named "tank" using the specified DAX device.

    apool list
        Displays a table of all available pools.

    apool list -p json
        Displays pool information in JSON format, suitable for scripting.

    apool destroy mypool
        Destroys the pool named "mypool".

SEE ALSO
    aethelfs(7)

##### `afs(8)`

```text
AFS(1)                    User Commands                  AFS(1)

NAME
    afs - AethelFS filesystem management utility

SYNOPSIS
    afs <subcommand> [arguments]

DESCRIPTION
    The afs utility manages AethelFS filesystems. A filesystem is created within a pool and is the entity that gets mounted and presented to the user.

SUBCOMMANDS

    create poolname
        Creates a new filesystem on the specified poolname. This initializes the filesystem superblock, allocation bitmaps, and root directory. The filesystem will implicitly take on the name of the pool.

    destroy [-f] poolname
        Destroys the filesystem on the specified poolname. This erases the filesystem superblock. The underlying pool remains intact.

        -f
            Force destruction even if the filesystem is currently mounted.

    list [-H] [-o field,...] [-p format]
        Lists all AethelFS filesystems.

        -H
            Do not print headers.

        -o field,...
            A comma-separated list of fields to display. Available fields are: name, used, available, mountpoint.

        -p format
            Print output in a specific format. Supported formats are table (default) and json.

    mount poolname mountpoint
        Mounts the filesystem from the specified poolname at the directory mountpoint. This command starts the aethelfsd FUSE daemon. The mountpoint must exist.

    unmount mountpoint
        Unmounts the AethelFS filesystem at the given mountpoint.

EXAMPLES

    afs create tank
        Creates a new filesystem within the "tank" pool.

    sudo afs mount tank /mnt/fast
        Mounts the "tank" filesystem at "/mnt/fast".

    afs list
        Displays a table of all AethelFS filesystems, their usage, and where they are mounted.

    sudo afs unmount /mnt/fast
        Unmounts the filesystem.

SEE ALSO
    apool(1), aethelfsd(8)
```

#### 8. User Flows

**Flow 1: Creating and Using a File-Backed Filesystem**

1.  **Admin:**  `sudo apool create -s 512M devpool /var/aethelfs/dev.img`
    
2.  **System:**  `apool` creates a 512MB file and writes the AethelFS pool label to the first block.
    
3.  **Admin:**  `sudo afs create devpool`
    
4.  **System:**  `afs` finds `/var/aethelfs/dev.img` associated with `devpool`, memory-maps it, and writes the filesystem superblock, bitmaps, and inode table.
    
5.  **Admin:**  `mkdir /mnt/dev`
    
6.  **Admin:**  `sudo afs mount devpool /mnt/dev`
    
7.  **System:**  `afs` launches the `aethelfsd` FUSE daemon. The daemon `mmap`s the file, reads the superblock, and tells the kernel it's ready to serve requests for `/mnt/dev`.
    
8.  **User:**  `ls -l /mnt/dev`
    
9.  **System:** The kernel sends a `readdir` request to the FUSE daemon. The daemon reads its directory entry structure for the root inode and returns the results.
    
10.  **Admin:**  `sudo afs unmount /mnt/dev`
    
11.  **System:** The FUSE daemon is terminated, and the filesystem is no longer accessible. The data remains in the backing file.
    

#### 9. FUSE Integration Explained

When `afs mount` is called, it executes the main `aethelfsd` process. The flow is:

1.  `aethelfsd` receives the pool name and mountpoint as arguments.
    
2.  It uses the pool name to find the associated device/file (by reading pool labels).
    
3.  It opens the device/file and `mmap()`s the entire pool into its own virtual address space.
    
4.  It initializes its internal state by reading the Superblock and other metadata from the memory-mapped region.
    
5.  It invokes the FUSE library, passing it a struct that satisfies the FUSE filesystem interface. Methods like `Getattr`, `Read`, `Write`, etc., are implemented on this struct.
    
6.  These implemented methods perform all their work (allocating blocks, writing data, updating inodes) directly in the memory-mapped region. For persistence on PMem, `msync(MS_SYNC)` is called after critical metadata updates.
    
7.  The kernel's VFS now directs all file operations under the mountpoint to the `aethelfsd` process.
    

#### 10. Minimal Viable Product (MVP) Recommendations

To get a prototype working quickly, focus on the absolute essentials.

1.  **File-Backed Pools First:** Fully implement and test the entire workflow using file-backed devices. This defers CXL hardware-specific issues.
    
2.  **Core `apool` Commands:**  `create -s`, `list`, `destroy`.
    
3.  **Core `afs` Commands:**  `create`, `mount`, `unmount`. `list` is a bonus.
    
4.  **Simplified Allocator:** Implement a simple **bitmap allocator** first. It's the most straightforward approach and is sufficient for an MVP. You can replace the data block allocation bitmap with a more efficient Buddy Allocator post-MVP.
    
5.  **Essential FUSE Operations:**
    
    -   `Statfs` (to satisfy `df` and prove your metadata tracking works)
        
    -   `Getattr`
        
    -   `Lookup`, `Readdir`
        
    -   `Create`, `Mknod`
        
    -   `Read`, `Write`
        
    -   `Unlink`, `Rmdir`
        
    -   `Truncate`
        
6.  **Skip for MVP:**
    
    -   Hard links (`link`) and symlinks (`symlink`).
        
    -   Complex permissions enforcement (`chown`, `chmod`). You can just return success and use default permissions initially.
        
    -   Extended attributes.
        
    -   Pool resizing or advanced management features.
        

This tightly scoped MVP provides a complete end-to-end user flow, proves the architectural concept, and builds a solid foundation for future enhancements.
