package main

import (
    "flag"
    "log"
    "os"
    "os/signal"

    "bazil.org/fuse"
    // bazilfs "bazil.org/fuse/fs"  // Rename to avoid conflict
    "aethelfs/internal/dax"
    "aethelfs/internal/fs"       // Your filesystem package
)

func main() {
    // Parse command line arguments
    flag.Parse()
    if flag.NArg() != 2 {
        log.Fatal("Usage: aethelfsd <dax-device> <mountpoint>")
    }

    daxPath := flag.Arg(0)
    mountpoint := flag.Arg(1)

    // Open the DAX device
    device, err := dax.NewDevice(daxPath)
    if err != nil {
        log.Fatalf("Failed to open DAX device: %v", err)
    }
    defer device.Close()

    // Set up FUSE connection
    c, err := fuse.Mount(
        mountpoint,
        fuse.FSName("aethelfs"),
        fuse.Subtype("aethelfsd"),
    )
    if err != nil {
        log.Fatalf("Failed to mount FUSE filesystem: %v", err)
    }
    defer c.Close()

    // Initialize the filesystem with the DAX device
    filesystem, err := fs.NewFilesystem(device)
    if err != nil {
        log.Fatalf("Failed to create filesystem: %v", err)
    }

    // Serve the filesystem
    if err := fs.Serve(c, filesystem); err != nil {
        log.Fatalf("Failed to serve FUSE filesystem: %v", err)
    }

    // Wait for the FUSE server to exit properly
    log.Println("Filesystem mounted successfully. Press Ctrl+C to exit.")
    
    // Set up signal handling for clean shutdown
    signalCh := make(chan os.Signal, 1)
    signal.Notify(signalCh, os.Interrupt)
    <-signalCh
    
    log.Println("Unmounting filesystem...")
    fuse.Unmount(mountpoint)
}