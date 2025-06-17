package cache

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

// CLWB flushes the cache line for the given address to memory
func CLWB(addr unsafe.Pointer) {
	// Use CLFLUSH if available
	if cpu.X86.HasSSE2 {
		asmCLFLUSH(addr)
	}
	// If no CPU support, do nothing - msync will be called instead
}

// Function declarations for assembly implementations
// These functions are implemented in clwb_amd64.s
func asmCLFLUSHOPT(addr unsafe.Pointer)
func asmCLFLUSH(addr unsafe.Pointer)
func asmCLWB(addr unsafe.Pointer)

// EnsureDataConsistency ensures data is flushed to memory
func EnsureDataConsistency(addr unsafe.Pointer, size int) {
	cacheLineSize := 64 // Common cache line size

	// Flush each cache line in the range
	for i := 0; i < size; i += cacheLineSize {
		CLWB(unsafe.Pointer(uintptr(addr) + uintptr(i)))
	}
}
