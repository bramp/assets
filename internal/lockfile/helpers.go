package lockfile

import "maps"

func (f *File) applyPending(upserts map[string]GeneratedRef, deletes map[string]struct{}) bool {
	if f == nil {
		return false
	}
	f.ensureDefaults()

	changed := false
	for path := range deletes {
		if _, ok := f.Files[path]; ok {
			delete(f.Files, path)
			changed = true
		}
	}
	for path, ref := range upserts {
		f.Files[path] = ref
		changed = true
	}
	return changed
}

func (f *File) clone() *File {
	if f == nil {
		return New()
	}

	clone := &File{
		Version:       f.Version,
		LastUpdatedAt: f.LastUpdatedAt,
		Files:         make(map[string]GeneratedRef, len(f.Files)),
	}
	clone.ensureDefaults()
	for outPath, ref := range f.Files {
		sourcesCopy := make(map[string]SourceRef, len(ref.Sources))
		maps.Copy(sourcesCopy, ref.Sources)

		var provenanceCopy *Provenance
		if ref.Provenance != nil {
			toolsCopy := make(map[string]string, len(ref.Provenance.Tools))
			maps.Copy(toolsCopy, ref.Provenance.Tools)
			chainCopy := append([]string(nil), ref.Provenance.CommandChain...)
			provenanceCopy = &Provenance{CommandChain: chainCopy, Tools: toolsCopy}
		}

		clone.Files[outPath] = GeneratedRef{
			Sources:    sourcesCopy,
			Provenance: provenanceCopy,
			SHA256:     ref.SHA256,
			SizeBytes:  ref.SizeBytes,
		}
	}

	return clone
}
