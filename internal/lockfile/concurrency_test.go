//nolint:testpackage // Intentional: deterministic checkpoint tests require unexported hooks.
package lockfile

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSession_SaveConflict_WithCheckpointInterleaving(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")

	writerA, err := Open(path)
	if err != nil {
		t.Fatalf("open writer A: %v", err)
	}
	defer func() { _ = writerA.Close() }()

	writerB, err := Open(path)
	if err != nil {
		t.Fatalf("open writer B: %v", err)
	}
	defer func() { _ = writerB.Close() }()

	writerA.UpsertOutput("assets/a.png", GeneratedRef{
		Sources:   map[string]SourceRef{"raw/a.svg": {SHA256: "aaa", SizeBytes: 1}},
		SHA256:    "111",
		SizeBytes: 10,
	})
	writerB.UpsertOutput("assets/b.png", GeneratedRef{
		Sources:   map[string]SourceRef{"raw/b.svg": {SHA256: "bbb", SizeBytes: 2}},
		SHA256:    "222",
		SizeBytes: 20,
	})

	aAtCheckpoint := make(chan struct{}, 1)
	releaseA := make(chan struct{})
	writerA.beforeReplace = func() {
		aAtCheckpoint <- struct{}{}
		<-releaseA
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var aErr error
	go func() {
		defer wg.Done()
		aErr = writerA.Save()
	}()

	<-aAtCheckpoint

	var bErr error
	go func() {
		defer wg.Done()
		bErr = writerB.Save()
	}()

	close(releaseA)
	wg.Wait()

	if aErr != nil {
		t.Fatalf("writer A save: %v", aErr)
	}
	if !errors.Is(bErr, ErrConflict) {
		t.Fatalf("expected ErrConflict for writer B, got: %v", bErr)
	}
}

func TestSession_SaveConflictThenRetry_DifferentOutputsConverge(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "assets.lock")

	writerA, err := Open(path)
	if err != nil {
		t.Fatalf("open writer A: %v", err)
	}
	defer func() { _ = writerA.Close() }()

	writerB, err := Open(path)
	if err != nil {
		t.Fatalf("open writer B: %v", err)
	}

	writerA.UpsertOutput("assets/a.png", GeneratedRef{
		Sources:   map[string]SourceRef{"raw/a.svg": {SHA256: "aaa", SizeBytes: 1}},
		SHA256:    "111",
		SizeBytes: 10,
	})
	writerB.UpsertOutput("assets/b.png", GeneratedRef{
		Sources:   map[string]SourceRef{"raw/b.svg": {SHA256: "bbb", SizeBytes: 2}},
		SHA256:    "222",
		SizeBytes: 20,
	})

	aAtCheckpoint := make(chan struct{}, 1)
	releaseA := make(chan struct{})
	writerA.beforeReplace = func() {
		aAtCheckpoint <- struct{}{}
		<-releaseA
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var aErr error
	go func() {
		defer wg.Done()
		aErr = writerA.Save()
	}()

	<-aAtCheckpoint

	var bErr error
	go func() {
		defer wg.Done()
		bErr = writerB.SaveWithRetry(3, time.Millisecond)
	}()

	close(releaseA)
	wg.Wait()

	if aErr != nil {
		t.Fatalf("writer A save: %v", aErr)
	}
	if bErr != nil {
		t.Fatalf("writer B save with retry: %v", bErr)
	}

	verify, err := Open(path)
	if err != nil {
		t.Fatalf("open verify session: %v", err)
	}
	defer func() { _ = verify.Close() }()

	got := verify.Snapshot().Files
	if _, ok := got["assets/a.png"]; !ok {
		t.Fatalf("expected assets/a.png after convergence, got keys: %#v", got)
	}
	if _, ok := got["assets/b.png"]; !ok {
		t.Fatalf("expected assets/b.png after convergence, got keys: %#v", got)
	}
}
