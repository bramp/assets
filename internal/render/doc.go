package render

// Package render resolves, plans, expands, and executes asset transformation
// pipelines.
//
// The package is split into small files so each phase is easier to read and
// test: resolution in resolve.go, execution in execute.go, command expansion
// in expand.go, and provenance collection in provenance.go.
