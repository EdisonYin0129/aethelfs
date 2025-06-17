package common

// Filesystem size and allocation constants
const (
	// Maximum total filesystem size (64GB)
	MaxFilesystemSize = int64(64 * 1024 * 1024 * 1024)

	// Default initial file allocation size (64KB)
	DefaultInitialFileSize = int64(64 * 1024)

	// Maximum single allocation size (2GB)
	MaxAllocationSize = int64(2 * 1024 * 1024 * 1024)

	// Metadata reservation size (1MB)
	MetadataReservationSize = int64(1 * 1024 * 1024)

	// Block alignment size (4KB - typical page size)
	BlockAlignmentSize = int64(4 * 1024)
)
