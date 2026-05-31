package lockfile_test

import (
	"time"

	"github.com/bramp/assets/internal/lockfile"
)

func ExampleSession_SaveWithRetry() {
	s, _ := lockfile.Open("assets.lock")
	defer s.Close()

	s.UpsertOutput("assets/images/logo_128.png", lockfile.GeneratedRef{
		Sources: map[string]lockfile.SourceRef{
			"raw/logo.svg": {SHA256: "abc123", SizeBytes: 1111},
		},
		SHA256:    "def456",
		SizeBytes: 2048,
		Provenance: &lockfile.Provenance{
			CommandChain: []string{"resvg --width 128 raw/logo.svg assets/images/logo_128.png"},
			Tools:        map[string]string{"resvg": "0.42.0"},
		},
	})
	_ = s.SaveWithRetry(5, 10*time.Millisecond)

	// Output:
}

func ExampleSession_DeleteOutput() {
	s, _ := lockfile.Open("assets.lock")
	defer s.Close()

	s.DeleteOutput("assets/images/old.png")
	_ = s.Save()

	// Output:
}
