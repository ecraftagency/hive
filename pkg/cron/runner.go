package cron

import (
	"context"
	"strings"
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

	// Map để track running jobs
	runningJobs := make(map[string]bool)

	// 1. Lấy danh sách tất cả running jobs
	list, _, err := jobs.List(nil)
	if err == nil {
		for _, j := range list {
			if j == nil || j.ID == "" {
				continue
			}
			allocs, _, aerr := jobs.Allocations(j.ID, false, nil)
			if aerr != nil {
				continue
			}
			for _, s := range allocs {
				if s != nil && s.ClientStatus == "running" {
					runningJobs[j.ID] = true
					break
				}
			}
		}
	}

	// 2. Xử lý từng room
	for _, rid := range roomIDs {
		st, _ := r.store.GetRoomState(ctx, rid)

		// Nếu room đã terminal: đảm bảo job đã dừng
		if st != nil && (st.Status == "DEAD" || st.Status == "FULFILLED") {
			if runningJobs[rid] {
				// Job vẫn chạy → dừng ngay
				_, _, _ = jobs.Deregister(rid, false, nil)
			}
			continue
		}

		// Kiểm tra job có tồn tại và running không
		isRunning := runningJobs[rid]

		if !isRunning {
			// Job không chạy
			if st == nil || now-st.CreatedAt > r.opts.GraceSeconds {
				_ = r.store.DeleteRoomState(ctx, rid)
				continue
			}

			// ACTIVED không còn chạy: server crash -> DEAD
			if st != nil && st.Status == "ACTIVED" {
				_ = r.store.SaveRoomState(ctx, store.RoomState{
					RoomID: rid, Players: st.Players, CreatedAt: st.CreatedAt,
					Status: "DEAD", FailReason: "server_crash", DeadAt: now,
				})
				continue
			}

			// OPENED quá lâu: đánh dấu DEAD (alloc_timeout)
			if st != nil && st.Status == "OPENED" && now-st.CreatedAt > r.opts.GraceSeconds {
				_ = r.store.SaveRoomState(ctx, store.RoomState{
					RoomID: rid, Players: st.Players, CreatedAt: st.CreatedAt,
					Status: "DEAD", FailReason: "alloc_timeout", DeadAt: now,
				})
				continue
			}
		}
	}

	// 3. Dừng các game server job không có room tương ứng (stray jobs)
	for jobID := range runningJobs {
		// Chỉ xử lý game server jobs (có prefix)
		if !strings.HasPrefix(jobID, r.opts.JobPrefix) {
			continue
		}
		found := false
		for _, rid := range roomIDs {
			if rid == jobID {
				st, _ := r.store.GetRoomState(ctx, rid)
				if st != nil && st.Status == "ACTIVED" {
					found = true
					break
				}
			}
		}
		if !found {
			// Game server job không có room ACTIVED tương ứng → dừng
			_, _, _ = jobs.Deregister(jobID, false, nil)
		}
	}
}

// stopStrayJobs removed - keep stopped jobs for log inspection
// Only sync Redis state to match Nomad running jobs (one-way consistency)
