package lockfile

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"time"
)

// Session tracks pending lockfile mutations and persists them atomically.
type Session struct {
	path   string
	closed bool

	base     *File
	baseHash hashDigest

	// beforeReplace, when set, is invoked while holding the advisory lock and
	// after hash-CAS validation, immediately before rename. It exists only to
	// enable deterministic interleavings in tests.
	beforeReplace func()

	pendingUpsert map[string]GeneratedRef
	pendingDelete map[string]struct{}
}

// ErrConflict indicates the lockfile changed between read and save.
var ErrConflict = errors.New("lockfile conflict")

// ConflictError reports an optimistic write conflict with expected/actual hashes.
type ConflictError struct {
	Path         string
	ExpectedHash hashDigest
	ActualHash   hashDigest
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf(
		"%s: content conflict (expected hash=%x, actual hash=%x)",
		e.Path,
		e.ExpectedHash[:],
		e.ActualHash[:],
	)
}

// Is matches ConflictError against ErrConflict.
func (e *ConflictError) Is(target error) bool {
	return target == ErrConflict
}

// Open loads lockfile state from path and returns a mutable session handle.
func Open(path string) (*Session, error) {
	f, h, err := loadUnlocked(path)
	if err != nil {
		return nil, err
	}

	return &Session{
		path:          path,
		base:          f,
		baseHash:      h,
		pendingUpsert: make(map[string]GeneratedRef),
		pendingDelete: make(map[string]struct{}),
	}, nil
}

// Snapshot returns a copy of current session state with pending changes applied.
func (s *Session) Snapshot() *File {
	if s == nil {
		return New()
	}
	merged := s.base.clone()
	merged.applyPending(s.pendingUpsert, s.pendingDelete)
	return merged
}

// UpsertOutput queues one generated-output record to be written on Save.
func (s *Session) UpsertOutput(outputPath string, ref GeneratedRef) {
	if s == nil {
		return
	}

	sourcesCopy := maps.Clone(ref.Sources)

	var provenanceCopy *Provenance
	if ref.Provenance != nil {
		toolsCopy := maps.Clone(ref.Provenance.Tools)
		chainCopy := append([]string(nil), ref.Provenance.CommandChain...)
		provenanceCopy = &Provenance{CommandChain: chainCopy, Tools: toolsCopy}
	}

	s.pendingUpsert[outputPath] = GeneratedRef{
		Sources:    sourcesCopy,
		Provenance: provenanceCopy,
		SHA256:     ref.SHA256,
		SizeBytes:  ref.SizeBytes,
	}
	delete(s.pendingDelete, outputPath)
}

// DeleteOutput queues removal of one generated-output record on Save.
func (s *Session) DeleteOutput(outputPath string) {
	if s == nil {
		return
	}

	delete(s.pendingUpsert, outputPath)
	s.pendingDelete[outputPath] = struct{}{}
}

// Save atomically applies queued mutations using optimistic hash validation.
func (s *Session) Save() error {
	if s == nil {
		return nil
	}
	if s.closed {
		return os.ErrClosed
	}
	if len(s.pendingUpsert) == 0 && len(s.pendingDelete) == 0 {
		return nil
	}

	next := s.base.clone()
	applied := next.applyPending(s.pendingUpsert, s.pendingDelete)
	if !applied {
		s.clearPending()
		return nil
	}

	next.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeCopyReplaceIfHashMatches(s.path, s.baseHash, next, s.beforeReplace); err != nil {
		return err
	}

	nextHash, err := hashCanonical(next)
	if err != nil {
		return fmt.Errorf("hash canonical lockfile: %w", err)
	}
	s.base = next
	s.baseHash = nextHash
	s.clearPending()
	return nil
}

// SaveWithRetry retries Save with exponential backoff when optimistic conflicts
// are detected. Pending mutations remain queued across retries.
func (s *Session) SaveWithRetry(maxAttempts int, initialDelay time.Duration) error {
	if maxAttempts < 1 {
		return errors.New("maxAttempts must be >= 1")
	}
	if initialDelay <= 0 {
		initialDelay = 10 * time.Millisecond
	}

	delay := initialDelay
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := s.Save()
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrConflict) {
			return err
		}
		if attempt == maxAttempts {
			return fmt.Errorf("save lockfile %q after %d attempts: %w", s.path, attempt, err)
		}

		base, baseHash, loadErr := loadUnlocked(s.path)
		if loadErr != nil {
			return fmt.Errorf("reload lockfile %q for retry: %w", s.path, loadErr)
		}
		s.base = base
		s.baseHash = baseHash

		time.Sleep(delay)
		delay *= 2
		if delay > time.Second {
			delay = time.Second
		}
	}

	return nil
}

// Close marks the session closed and prevents further saves.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.closed = true
	return nil
}

func (s *Session) clearPending() {
	for k := range s.pendingUpsert {
		delete(s.pendingUpsert, k)
	}
	for k := range s.pendingDelete {
		delete(s.pendingDelete, k)
	}
}
