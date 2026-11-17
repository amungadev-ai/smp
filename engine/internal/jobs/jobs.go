// Package jobs runs deployment, rollback and restart workflows on background
// workers. This is the heart of M01: pull → run → health → Caddy → release
// history, with every step recorded on the job.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"drevanta/engine/internal/healthcheck"
	"drevanta/engine/internal/model"
	"drevanta/engine/internal/proxy"
	"drevanta/engine/internal/runtime"
	"drevanta/engine/internal/store"
)

const (
	pullTimeout     = 10 * time.Minute
	startTimeout    = 2 * time.Minute
	healthAttempts  = 20
	healthPause     = 500 * time.Millisecond
	workerCount     = 2
	releasePerProject = 50
)

// Engine ties the store, runtime, proxy and workers together.
type Engine struct {
	Version  string
	DataDir  string
	Store    *store.Store
	Docker   *runtime.Docker
	Caddy    *proxy.Caddy

	queue chan string
	mu    sync.Mutex
}

func New(version, dataDir string, st *store.Store, d *runtime.Docker, c *proxy.Caddy) *Engine {
	return &Engine{
		Version: version,
		DataDir: dataDir,
		Store:   st,
		Docker:  d,
		Caddy:   c,
		queue:   make(chan string, 128),
	}
}

// StartWorkers launches the background worker pool. Cancel ctx to stop.
func (e *Engine) StartWorkers(ctx context.Context) {
	for i := 0; i < workerCount; i++ {
		go e.worker(ctx)
	}
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// Enqueue creates a job record and pushes it onto the worker queue.
func (e *Engine) Enqueue(projectID, kind string) (*model.Job, error) {
	if _, ok := e.Store.Project(projectID); !ok {
		return nil, fmt.Errorf("project %s not found", projectID)
	}
	job := model.Job{
		ID:        newID(),
		ProjectID: projectID,
		Kind:      kind,
		Status:    model.JobQueued,
		Created:   model.Now(),
	}
	if err := e.Store.AddJob(job); err != nil {
		return nil, err
	}
	e.queue <- job.ID
	return &job, nil
}

func (e *Engine) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-e.queue:
			e.run(ctx, id)
		}
	}
}

func (e *Engine) jobLog(jobID, msg string) {
	e.Store.UpdateJob(jobID, func(j *model.Job) {
		j.Log = append(j.Log, model.JobLogLine{TS: model.Now(), Message: msg})
	})
}

func (e *Engine) fail(jobID, msg string) {
	e.Store.UpdateJob(jobID, func(j *model.Job) {
		j.Status = model.JobFailed
		j.Error = msg
		j.Finished = model.Now()
		j.Log = append(j.Log, model.JobLogLine{TS: model.Now(), Message: "FAILED: " + msg})
	})
}

func (e *Engine) succeed(jobID string) {
	e.Store.UpdateJob(jobID, func(j *model.Job) {
		j.Status = model.JobSucceeded
		j.Finished = model.Now()
	})
}

func (e *Engine) run(ctx context.Context, jobID string) {
	if ctx.Err() != nil {
		return // shutting down — job stays queued for the next boot
	}
	job, ok := e.Store.Job(jobID)
	if !ok {
		return
	}
	e.Store.UpdateJob(jobID, func(j *model.Job) { j.Status = model.JobRunning; j.Started = model.Now() })

	project, ok := e.Store.Project(job.ProjectID)
	if !ok {
		e.fail(jobID, "project vanished from state")
		return
	}

	switch job.Kind {
	case model.JobDeploy:
		e.runDeploy(ctx, jobID, project, project.Image, "")
	case model.JobRollback:
		e.runRollback(ctx, jobID, project)
	case model.JobRestart:
		e.runRestart(ctx, jobID, project)
	default:
		e.fail(jobID, "unknown job kind "+job.Kind)
	}
}

// runDeploy executes one release of image. rollbackReuseID names an existing
// release to deploy again (rollback path).
func (e *Engine) runDeploy(ctx context.Context, jobID string, project model.Project, image string, reuseReleaseID string) {
	tag := "latest"
	for i := len(image) - 1; i > 0; i-- {
		if image[i] == '/' {
			break
		}
		if image[i] == ':' {
			tag = image[i+1:]
			image = image[:i]
			break
		}
	}

	release := model.Release{
		ID:        newID(),
		ProjectID: project.ID,
		Image:     image,
		Tag:       tag,
		Created:   model.Now(),
		Status:    model.ReleaseDeploying,
	}
	if err := e.Store.AddRelease(release); err != nil {
		e.fail(jobID, "persist release: "+err.Error())
		return
	}
	e.Store.UpdateJob(jobID, func(j *model.Job) { j.ReleaseID = release.ID })
	e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusDeploying })
	e.jobLog(jobID, fmt.Sprintf("pulling %s:%s", image, tag))

	pullCtx, cancel := context.WithTimeout(ctx, pullTimeout)
	defer cancel()
	if err := e.Docker.Pull(pullCtx, release.Image+":"+tag); err != nil {
		e.Store.UpdateRelease(release.ID, func(r *model.Release) { r.Status = model.ReleaseFailed })
		e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusFailed })
		e.fail(jobID, "pull: "+err.Error())
		return
	}

	e.jobLog(jobID, "starting container")
	runCtx, cancel2 := context.WithTimeout(ctx, startTimeout)
	defer cancel2()
	_, hostPort, err := e.Docker.EnsureRunning(runCtx, project)
	if err != nil {
		e.Store.UpdateRelease(release.ID, func(r *model.Release) { r.Status = model.ReleaseFailed })
		e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusFailed })
		e.fail(jobID, "start: "+err.Error())
		return
	}
	e.jobLog(jobID, fmt.Sprintf("container up — health probing port %d", hostPort))

	healthy, latency, herr := e.checkHealth(ctx, hostPort)
	if !healthy {
		e.Store.UpdateRelease(release.ID, func(r *model.Release) {
			r.Status = model.ReleaseUnhealthy
			r.HealthOK = &healthy
		})
		e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusFailed })
		e.fail(jobID, "health check failed: "+safe(herr))
		return
	}
	e.Store.UpdateRelease(release.ID, func(r *model.Release) {
		r.Status = model.ReleaseHealthy
		r.HealthOK = &healthy
		r.HealthLatencyMS = latency
	})
	e.jobLog(jobID, fmt.Sprintf("healthy in %d ms", latency))

	if reuseReleaseID != "" {
		e.Store.UpdateRelease(reuseReleaseID, func(r *model.Release) { r.Status = model.ReleaseRolledBack })
	}

	// routing: keep Caddy in sync with every domain that points at a project
	if project.Domain != "" {
		e.jobLog(jobID, "updating Caddy routing")
		e.syncCaddy(jobID)
	}

	releaseCount := len(e.Store.Releases(project.ID)) // read BEFORE UpdateProject (store locks are not reentrant)
	e.Store.UpdateProject(project.ID, func(p *model.Project) {
		p.Status = model.StatusRunning
		p.CurrentRelease = release.ID
		p.ReleaseCount = releaseCount
	})
	e.succeed(jobID)
}

// runRollback redeploys the previous healthy release.
func (e *Engine) runRollback(ctx context.Context, jobID string, project model.Project) {
	releases := e.Store.Releases(project.ID)
	var target *model.Release
	for i := len(releases) - 1; i >= 0; i-- {
		r := releases[i]
		if r.ID != project.CurrentRelease && (r.Status == model.ReleaseHealthy || r.Status == model.ReleaseRolledBack) {
			target = &r
			break
		}
	}
	if target == nil {
		e.fail(jobID, "no previous healthy release to roll back to")
		return
	}
	e.jobLog(jobID, fmt.Sprintf("rolling back to %s:%s", target.Image, target.Tag))
	e.runDeploy(ctx, jobID, project, target.Image+":"+target.Tag, target.ID)
}

// runRestart restarts the container in place and re-checks health.
func (e *Engine) runRestart(ctx context.Context, jobID string, project model.Project) {
	e.jobLog(jobID, "restarting container")
	if err := e.Docker.Restart(project.Name); err != nil {
		e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusFailed })
		e.fail(jobID, "restart: "+err.Error())
		return
	}
	_, hostPort, err := e.Docker.EnsureRunning(ctx, project)
	if err != nil {
		e.fail(jobID, "after restart: "+err.Error())
		return
	}
	healthy, latency, herr := e.checkHealth(ctx, hostPort)
	if !healthy {
		e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusFailed })
		e.fail(jobID, "health after restart failed: "+safe(herr))
		return
	}
	e.Store.UpdateProject(project.ID, func(p *model.Project) { p.Status = model.StatusRunning })
	e.jobLog(jobID, fmt.Sprintf("restart healthy in %d ms", latency))
	e.succeed(jobID)
}

// checkHealth is real in docker mode, simulated in mock mode (no container
// actually listens on the port in the lab).
func (e *Engine) checkHealth(ctx context.Context, hostPort int) (bool, int64, error) {
	if e.Docker.Mode() == runtime.ModeMock {
		time.Sleep(150 * time.Millisecond)
		return true, 12, nil
	}
	hCtx, cancel := context.WithTimeout(ctx, healthAttempts*healthPause+5*time.Second)
	defer cancel()
	ok, latency, err := healthcheck.WaitUntilHealthy(hCtx, hostPort, healthAttempts, healthPause)
	if ok {
		return true, latency, nil
	}
	return false, latency, err
}

// syncCaddy reapplies routing from engine state and mirrors the Caddyfile.
func (e *Engine) syncCaddy(jobID string) {
	domains := e.Store.Domains()
	projects := e.Store.Projects()
	if err := e.Caddy.ApplyConfig(domains, projects); err != nil {
		e.jobLog(jobID, "caddy warning: "+err.Error())
		return
	}
	if e.Caddy.Mode() == proxy.ModeCaddy && e.DataDir != "" {
		cfg := proxy.BuildCaddyfile(domains, projects)
		path := e.DataDir + "/Caddyfile"
		if err := writeFile(path, []byte(cfg)); err != nil {
			e.jobLog(jobID, "caddyfile mirror failed: "+err.Error())
		}
	}
}

func safe(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}
