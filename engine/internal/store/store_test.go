package store

import (
	"os"
	"testing"

	"drevanta/engine/internal/model"
)

func readDirEntries(dir string) ([]os.DirEntry, error) {
	return os.ReadDir(dir)
}

func TestPersistenceRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	p := model.Project{ID: "web-abc", Name: "web", Image: "nginx", Port: 80, Status: model.StatusRunning, Created: model.Now()}
	if err := s.AddProject(p); err != nil {
		t.Fatalf("add project: %v", err)
	}
	rel := model.Release{ID: "rel1", ProjectID: "web-abc", Image: "nginx", Tag: "alpine", Created: model.Now(), Status: model.ReleaseHealthy}
	if err := s.AddRelease(rel); err != nil {
		t.Fatalf("add release: %v", err)
	}
	d := model.Domain{ID: "d1", Name: "app.example.com", ProjectID: "web-abc", Scheme: "https", Created: model.Now(), Status: model.DomainActive}
	if err := s.AddDomain(d); err != nil {
		t.Fatalf("add domain: %v", err)
	}

	// reopen from disk — everything must survive
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := s2.Project("web-abc")
	if !ok || got.Image != "nginx" || got.Port != 80 {
		t.Fatalf("project did not survive reload: %+v", got)
	}
	if rels := s2.Releases("web-abc"); len(rels) != 1 || rels[0].Status != model.ReleaseHealthy {
		t.Fatalf("releases did not survive reload: %+v", rels)
	}
	if ds := s2.Domains(); len(ds) != 1 || ds[0].Name != "app.example.com" {
		t.Fatalf("domains did not survive reload: %+v", ds)
	}
}

func TestAtomicSaveLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	if err := s.AddProject(model.Project{ID: "x", Name: "x", Image: "img", Port: 80, Created: model.Now()}); err != nil {
		t.Fatal(err)
	}
	entries, _ := readDirEntries(dir)
	for _, e := range entries {
		if e.Name() == "state.json.tmp" {
			t.Fatalf("temp file left behind: %v", entries)
		}
	}
}

func TestDuplicateDomainRejected(t *testing.T) {
	s, _ := Open(t.TempDir())
	d := model.Domain{ID: "1", Name: "a.example.com", Scheme: "https", Created: model.Now(), Status: model.DomainPending}
	if err := s.AddDomain(d); err != nil {
		t.Fatal(err)
	}
	d2 := model.Domain{ID: "2", Name: "a.example.com", Scheme: "https", Created: model.Now(), Status: model.DomainPending}
	if err := s.AddDomain(d2); err == nil {
		t.Fatal("duplicate domain accepted")
	}
}

func TestJobHistoryBounded(t *testing.T) {
	s, _ := Open(t.TempDir())
	for i := 0; i < 250; i++ {
		if err := s.AddJob(model.Job{ID: string(rune('a'+i%26)) + string(rune('a'+i/26)), ProjectID: "p", Kind: model.JobDeploy, Status: model.JobQueued, Created: model.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(s.Jobs("")); n > 200 {
		t.Fatalf("job history not bounded: %d", n)
	}
}
