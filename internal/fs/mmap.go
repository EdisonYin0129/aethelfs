package fs

import (
    "golang.org/x/sys/unix"
    "os"
    // "unsafe"
)

// MmapFile represents a memory-mapped file.
type MmapFile struct {
    File   *os.File
    Length int
    Data   []byte
}

// Mmap creates a memory mapping for the specified file.
func Mmap(file *os.File, length int) (*MmapFile, error) {
    data, err := unix.Mmap(int(file.Fd()), 0, length, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
    if err != nil {
        return nil, err
    }

    return &MmapFile{
        File:   file,
        Length: length,
        Data:   data,
    }, nil
}

// Unmap unmaps the memory-mapped file.
func (m *MmapFile) Unmap() error {
    if err := unix.Munmap(m.Data); err != nil {
        return err
    }
    return nil
}

// Flush flushes the memory-mapped data to the underlying device.
// Optionally, this can use msync() or CLWB for flushing.
func (m *MmapFile) Flush() error {
    // Use msync to flush changes to the underlying device.
    if err := unix.Msync(m.Data, unix.MS_SYNC); err != nil {
        return err
    }
    return nil
}