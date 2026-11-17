// Package store persists engine state as a single JSON document with atomic
// writes. M01 keeps everything local; PostgreSQL replaces this in milestone 05
// behind the same interface.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"drevanta/engine/internal/model"
)

const stateVersion = 1

type Store struct {
	mu    sync.RWMutex
	path  string
	state model.State
}

// Open loads state.json from dir, creating an empty store when absent.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{path: filepath.Join(dir, "state.json")}
	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.state = model.State{Version: stateVersion}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("read state: %w", err)
	default:
		if err := json.Unmarshal(raw, &s.state); err != nil {
			return nil, fmt.Errorf("parse state (copy %s aside to recover): %w", s.path, err)
		}
		if s.state.Version == 0 {
			s.state.Version = stateVersion
		}
	}
	return s, nil
}

// Path returns the backing file location (used by diagnostics).
func (s *Store) Path() string { return s.path }

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path) // atomic on POSIX
}

// Snapshot returns a deep copy of the current state for serialization.
func (s *Store) Snapshot() model.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := model.State{Version: s.state.Version}
	out.Projects = append(out.Projects, s.state.Projects...)
	out.Releases = append(out.Releases, s.state.Releases...)
	out.Jobs = append(out.Jobs, s.state.Jobs...)
	out.Domains = append(out.Domains, s.state.Domains...)
	return out
}

func (s *Store) Projects() []model.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]model.Project{}, s.state.Projects...)
	return out
}

func (s *Store) Releases(projectID string) []model.Release {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.Release
	for _, r := range s.state.Releases {
		if projectID == "" || r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) Jobs(projectID string) []model.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.Job
	for _, j := range s.state.Jobs {
		if projectID == "" || j.ProjectID == projectID {
			out = append(out, j)
		}
	}
	return out
}

func (s *Store) Job(id string) (model.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.state.Jobs {
		if j.ID == id {
			return j, true
		}
	}
	return model.Job{}, false
}

func (s *Store) Domains() []model.Domain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]model.Domain{}, s.state.Domains...)
	return out
}

func (s *Store) Project(id string) (model.Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.state.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return model.Project{}, false
}

func (s *Store) AddProject(p model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Projects = append(s.state.Projects, p)
	return s.saveLocked()
}

// UpdateProject applies fn to a project under lock and persists.
func (s *Store) UpdateProject(id string, fn func(*model.Project)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Projects {
		if s.state.Projects[i].ID == id {
			fn(&s.state.Projects[i])
			return s.saveLocked()
		}
	}
	return fmt.Errorf("project %s not found", id)
}

func (s *Store) RemoveProject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.state.Projects[:0]
	for _, p := range s.state.Projects {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	s.state.Projects = kept
	// detach domains that pointed at this project
	for i := range s.state.Domains {
		if s.state.Domains[i].ProjectID == id {
			s.state.Domains[i].ProjectID = ""
		}
	}
	return s.saveLocked()
}

func (s *Store) AddRelease(r model.Release) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Releases = append(s.state.Releases, r)
	return s.saveLocked()
}

func (s *Store) UpdateRelease(id string, fn func(*model.Release)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Releases {
		if s.state.Releases[i].ID == id {
			fn(&s.state.Releases[i])
			return s.saveLocked()
		}
	}
	return fmt.Errorf("release %s not found", id)
}

func (s *Store) AddJob(j model.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Jobs = append(s.state.Jobs, j)
	if len(s.state.Jobs) > 200 { // bounded history
		s.state.Jobs = s.state.Jobs[len(s.state.Jobs)-200:]
	}
	return s.saveLocked()
}

func (s *Store) UpdateJob(id string, fn func(*model.Job)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Jobs {
		if s.state.Jobs[i].ID == id {
			fn(&s.state.Jobs[i])
			return s.saveLocked()
		}
	}
	return fmt.Errorf("job %s not found", id)
}

func (s *Store) AddDomain(d model.Domain) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ex := range s.state.Domains {
		if ex.Name == d.Name {
			return fmt.Errorf("domain %s already exists", d.Name)
		}
	}
	s.state.Domains = append(s.state.Domains, d)
	return s.saveLocked()
}

func (s *Store) RemoveDomain(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.state.Domains[:0]
	found := false
	for _, d := range s.state.Domains {
		if d.ID == id {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	if !found {
		return fmt.Errorf("domain %s not found", id)
	}
	s.state.Domains = kept
	return s.saveLocked()
}
