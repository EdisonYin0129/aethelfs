package fs

import (
	"golang.org/x/sys/unix"
	"sync"
)

// Flusher provides methods to flush data from CPU caches to the underlying device.
type Flusher struct {
	mu sync.Mutex
}

// NewFlusher creates a new Flusher instance.
func NewFlusher() *Flusher {
	return &Flusher{}
}

// FlushMsync flushes the memory-mapped data to the underlying device using msync().
func (f *Flusher) FlushMsync(addr []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return unix.Msync(addr, unix.MS_SYNC)
}

// FlushCLWB flushes the memory-mapped data to the underlying device using CLWB.
func (f *Flusher) FlushCLWB(addr []byte) {
	// Use the CLWB instruction to flush the cache line for the given address.
	// This is a placeholder for the actual implementation that would use
	// assembly or a specific library to perform the CLWB operation.
	// Ensure that the address is aligned to the cache line size.
	// Implementation of CLWB would go here.
}
