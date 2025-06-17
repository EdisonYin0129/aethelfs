package fs

// DebugMode controls whether debug logging is enabled
var debugMode *bool = new(bool) // Initialize with false by default

// SetDebugMode sets the debug mode flag
func SetDebugMode(flag *bool) {
	if flag != nil {
		debugMode = flag
	}
}

// IsDebugEnabled returns whether debug mode is enabled
func IsDebugEnabled() bool {
	return debugMode != nil && *debugMode
}
