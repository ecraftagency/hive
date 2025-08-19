package cron

import (
	"context"
	"time"

	"hive/pkg/store"

	"github.com/hashicorp/nomad/api"
)

type Options struct {
	GraceSeconds int64
	JobPrefix    string
	Interval     time.Duration
}

type Runner struct {
	store   *store.Manager
	nomad   *api.Client
	opts    Options
	stopped chan struct{}
}

func New(storeMgr *store.Manager, nomadClient *api.Client, opts Options) *Runner {
	if opts.GraceSeconds <= 0 {
		opts.GraceSeconds = 60
	}
	if opts.JobPrefix == "" {
		opts.JobPrefix = "game-server-" // Default prefix, should be set from config
	}
	if opts.Interval <= 0 {
		opts.Interval = 10 * time.Second
	}
	return &Runner{store: storeMgr, nomad: nomadClient, opts: opts, stopped: make(chan struct{})}
}

// Start chạy vòng đồng bộ nền; dừng khi ctx.Done()
func (r *Runner) Start(ctx context.Context) {
	jobs := r.nomad.Jobs()
TickerLoop:
	for {
		select {
		case <-ctx.Done():
			break TickerLoop
		case <-time.After(r.opts.Interval):
			// Only sync Redis state to match Nomad running jobs (one-way consistency)
			// Keep stopped jobs for log inspection
			r.syncRooms(ctx, jobs)
		}
	}
	close(r.stopped)
}

func (r *Runner) syncRooms(ctx context.Context, jobs *api.Jobs) {
	roomIDs, err := r.store.ListRooms(ctx)
	if err != nil {
		return
	}
	now := time.Now().Unix()
	for _, rid := range roomIDs {
		st, _ := r.store.GetRoomState(ctx, rid)
		// Nếu job không tồn tại
		if _, _, jerr := jobs.Info(rid, nil); jerr != nil {
			if st == nil || now-st.CreatedAt > r.opts.GraceSeconds {
				_ = r.store.DeleteRoomState(ctx, rid)
			}
			continue
		}
		allocs, _, aerr := jobs.Allocations(rid, false, nil)
		if aerr != nil {
			if st == nil || now-st.CreatedAt > r.opts.GraceSeconds {
				_ = r.store.DeleteRoomState(ctx, rid)
			}
			continue
		}
		running := false
		for _, s := range allocs {
			if s.ClientStatus == "running" {
				running = true
				break
			}
		}
		if !running && (st == nil || now-st.CreatedAt > r.opts.GraceSeconds) {
			_ = r.store.DeleteRoomState(ctx, rid)
		}
	}
}

// stopStrayJobs removed - keep stopped jobs for log inspection
// Only sync Redis state to match Nomad running jobs (one-way consistency)
