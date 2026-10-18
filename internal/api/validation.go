package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/cfgtree"
	"github.com/runwisp/runwisp-manager/internal/runwispbin"
	"github.com/runwisp/runwisp-manager/internal/schema"
	"github.com/runwisp/runwisp-manager/internal/validate"
)

type ValidationAPI struct {
	Document schema.Document
	Options  validate.Options
	Watcher  *cfgtree.Watcher
}

func (a *ValidationAPI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/schema", a.schema)
	r.Post("/validate", a.validate)
	r.Get("/validate/current", a.current)
	return r
}
func (a *ValidationAPI) schema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.Document)
}
func (a *ValidationAPI) validate(w http.ResponseWriter, r *http.Request) {
	// Copy opts so concurrent requests cannot race on the shared Options.Mode field.
	opts := a.Options
	if opts.Mode == "" {
		opts.Mode = r.URL.Query().Get("mode")
	}
	var request struct {
		Files []struct {
			Path    string `json:"path"`
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		problem(w, http.StatusBadRequest, "invalid validation request")
		return
	}
	candidates := make([]validate.Candidate, 0, len(request.Files))
	for _, file := range request.Files {
		path := file.Path
		if path == "" && a.Watcher != nil {
			for _, known := range a.Watcher.Tree().Files {
				if known.ID == file.ID {
					path = known.Path
					break
				}
			}
		}
		if path == "" {
			problem(w, http.StatusBadRequest, "validation file path is required")
			return
		}
		candidates = append(candidates, validate.Candidate{Path: path, Content: []byte(file.Content)})
	}
	if len(candidates) == 0 && a.Watcher != nil {
		tree := a.Watcher.Tree()
		for _, file := range tree.Files {
			if file.Readable {
				data, err := os.ReadFile(file.Path)
				if err == nil {
					candidates = append(candidates, validate.Candidate{Path: file.Path, Content: data})
				}
			}
		}
	}
	result, err := validate.Validate(r.Context(), opts, candidates)
	if err != nil {
		problem(w, http.StatusInternalServerError, "validation failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (a *ValidationAPI) current(w http.ResponseWriter, r *http.Request) {
	// Invoke validate with mode forced to "full" without mutating shared state.
	// Override the query string so that validate's local opts copy picks up "full".
	r.URL.RawQuery = "mode=full"
	a.validate(w, r)
}

func NewValidationAPI(document schema.Document, watcher *cfgtree.Watcher, binary string, stateDir string) *ValidationAPI {
	root := ""
	if watcher != nil {
		root = watcher.Tree().Root
	}
	return &ValidationAPI{Document: document, Watcher: watcher, Options: validate.Options{Schema: document, Binary: runwispbin.Runner{Binary: binary}, Root: root, StateDir: stateDir}}
}
