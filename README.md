# AethelFS

**AethelFS** (pronounced “Ethel-F-S”) is a user-space filesystem designed for Compute Express Link (CXL) memory and other memory-like devices. It aims to provide a simple, high-performance solution for low-latency storage needs, with an interface modeled after familiar tools like ZFS.

## Project Vision

AethelFS’s goal is to make it easy for Linux administrators and developers to experiment with and deploy persistent or volatile memory-backed filesystems—without needing special kernel modules or privileged hardware. The entire system runs in userspace via FUSE, so development and testing can happen on any Linux machine using file-backed devices.

## Who is this for?

- **Linux System Administrators** managing high-performance storage for databases, caches, or scientific computing.
- **Developers** seeking direct, low-latency access to new memory tiers for testing or development—even without physical CXL hardware.

## Key Principles

- **Simplicity:** Avoid complexity—no RAID or advanced volume management.
- **Familiarity:** Command-line tools (`apool`, `afs`) modeled after ZFS for a gentle learning curve.
- **Userspace First:** Implemented entirely in userspace (FUSE).
- **Testability:** File-backed device simulation is a priority.

## Architecture Overview

AethelFS consists of:
- **Management Utilities:**  
  - `apool`: Create, list, and destroy storage pools (backed by files or devices).
  - `afs`: Create, mount, unmount, list, and destroy filesystems within those pools.
- **FUSE Daemon (`aethelfsd`):**  
  Handles all file operations, metadata management, and space allocation in userspace.

## Example Workflow

1. **Create a pool:**  
   `sudo apool create -s 1G testpool /tmp/aethelfs.img`
2. **Create a filesystem:**  
   `sudo afs create testpool`
3. **Create a mount point:**  
   `mkdir /mnt/test`
4. **Mount the filesystem:**  
   `sudo afs mount testpool /mnt/test`
5. **Use it as a regular filesystem!**

## Getting Started

- No CXL hardware required—use file-backed pools.
- Focus on the `apool` and `afs` tools for core management tasks.
- The MVP (minimum viable product) implements basic creation, mounting, file operations, and teardown.
