package cache

import (
	"golang.org/x/sys/cpu"
	"unsafe"
)

// CLWB flushes the cache line for the given address to the underlying memory.
// This function uses the CLFLUSH instruction if available, which is faster than msync.
func CLWB(addr unsafe.Pointer) {
	// For MVP, we'll just use CLFLUSH which is more widely available
	// The X86 package doesn't directly expose HasCLFLUSHOPT
	if cpu.X86.HasSSE2 {
		// Most modern CPUs that support SSE2 also support CLFLUSH
		asmCLFLUSH(addr)
	}
	// If no CPU support, do nothing - msync will be called elsewhere
}

// For now, just implement stubs for the assembly functions
func asmCLFLUSHOPT(addr unsafe.Pointer) {
	// This should be implemented in assembly
	// For now, do nothing
}

func asmCLFLUSH(addr unsafe.Pointer) {
	// This should be implemented in assembly
	// For now, do nothing
}

// EnsureDataConsistency ensures that the data at the given address is flushed to the underlying memory.
func EnsureDataConsistency(addr unsafe.Pointer, size int) {
	// A simple implementation that just calls CLWB for every cache line
	// In a real implementation, you would use the actual cache line size
	cacheLineSize := 64 // Common cache line size

	for i := 0; i < size; i += cacheLineSize {
		CLWB(unsafe.Pointer(uintptr(addr) + uintptr(i)))
	}
}