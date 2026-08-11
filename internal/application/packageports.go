package application

// SafeRemoveAll exports the verified instance-directory removal used by
// instance and package cleanup wiring.
func SafeRemoveAll(path, dataRoot, marker string) error {
	return safeRemoveAll(path, dataRoot, marker)
}
