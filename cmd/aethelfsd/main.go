package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"aethelfs/internal/common"
	"aethelfs/internal/dax"
	"aethelfs/internal/fs"

	"bazil.org/fuse"
)

// Global debug flag that can be accessed from other packages
var debugMode *bool

func main() {
	// Define command-line flags
	debugMode = flag.Bool("debug", false, "Enable debug mode with verbose logging")

	// Parse command line arguments
	flag.Parse()

	// Make the debug flag available to the fs package
	fs.SetDebugMode(debugMode)

	// Check if debug mode is enabled
	if *debugMode {
		log.Println("Debug mode enabled - verbose logging activated")
		// Set up additional logging configuration here
	}

	// Check arguments (adjusted to account for possible flags)
	args := flag.Args()
	if len(args) != 2 {
		log.Fatal("Usage: aethelfsd [-debug] <dax-device> <mountpoint>")
	}

	daxPath := args[0]
	mountpoint := args[1]

	// Open the DAX device
	device, err := dax.NewDevice(daxPath)
	if err != nil {
		log.Fatalf("Failed to open DAX device: %v", err)
	}
	defer device.Close()

	// Build mount options with optimized settings
	opts := []fuse.MountOption{
		fuse.FSName("aethelfs"),
		fuse.Subtype("aethelfsd"),
		fuse.AllowOther(),
		fuse.MaxReadahead(4 * 1024 * 1024), // 4MB readahead
		fuse.AsyncRead(),                   // Enable asynchronous reads
		fuse.WritebackCache(),              // Enable write caching
		fuse.MaxBackground(64),             // Increase concurrent operations
	}

	// Enable low‑level FUSE package logging
	if *debugMode {
		fuse.Debug = func(msg interface{}) {
			log.Printf("FUSE: %v", msg)
		}
	}

	// Set up FUSE connection
	c, err := fuse.Mount(mountpoint, opts...)
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
	log.Printf("Filesystem mounted successfully at %s (%.2f GB available). Press Ctrl+C to exit.",
		mountpoint, float64(common.MaxFilesystemSize)/(1024*1024*1024))
	// Set up signal handling for clean shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	log.Println("Unmounting filesystem...")
	err = fuse.Unmount(mountpoint)
	if err != nil {
		log.Printf("Warning: Failed to unmount cleanly: %v", err)
		log.Println("You may need to run 'fusermount -u " + mountpoint + "' manually")
	}
}
