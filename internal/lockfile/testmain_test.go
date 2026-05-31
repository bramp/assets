package lockfile_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	cleanupExampleArtifacts()
	exitCode := m.Run()
	cleanupExampleArtifacts()
	os.Exit(exitCode)
}

func cleanupExampleArtifacts() {
	_ = os.Remove("assets.lock")
	_ = os.Remove("assets.lock.lock")
}
