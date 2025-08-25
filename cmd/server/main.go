package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"hive/pkg/svrsdk"
	"hive/pkg/ui"

	"github.com/gin-gonic/gin"
)

type playerInfo struct {
	PlayerID string `json:"player_id"`
	State    string `json:"state"`
	LastSeen int64  `json:"last_seen_unix"`
}

type playerStore struct {
	mu       sync.RWMutex
	lastSeen map[string]time.Time
}

func newPlayerStore() *playerStore { return &playerStore{lastSeen: make(map[string]time.Time)} }

func (ps *playerStore) heartbeat(pid string) {
	ps.mu.Lock()
	ps.lastSeen[pid] = time.Now()
	ps.mu.Unlock()
}

func (ps *playerStore) size() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.lastSeen)
}

func (ps *playerStore) snapshot(now time.Time, ttl time.Duration) []playerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	res := make([]playerInfo, 0, len(ps.lastSeen))
	for id, ts := range ps.lastSeen {
		state := "connected"
		if now.Sub(ts) > ttl {
			state = "disconnected"
		}
		res = append(res, playerInfo{PlayerID: id, State: state, LastSeen: ts.Unix()})
	}
	return res
}

func (ps *playerStore) anyDisconnected(now time.Time, ttl time.Duration) (bool, string) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for id, ts := range ps.lastSeen {
		if now.Sub(ts) > ttl {
			return true, id
		}
	}
	return false, ""
}

// countActive returns number of players with heartbeat within TTL
func (ps *playerStore) countActive(now time.Time, ttl time.Duration) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	active := 0
	for _, ts := range ps.lastSeen {
		if now.Sub(ts) <= ttl {
			active++
		}
	}
	return active
}

func ginLog(format string, args ...interface{}) { fmt.Fprintf(gin.DefaultWriter, format+"\n", args...) }

func main() {
	// Parse config via SDK (ENV first, then flags)
	cfg := svrsdk.FromEnvOrArgs(os.Args)
	serverPort := cfg.ServerPort
	roomID := cfg.RoomID
	bearer := cfg.Token
	agentBase := cfg.AgentBaseURL

	// Validate required arguments
	if serverPort == "" {
		ginLog("missing -serverPort argument")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(serverPort); err != nil {
		ginLog("invalid serverPort: %s", serverPort)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	ginLog("startup args: serverPort=%s room=%s bearer=%s agent=%s", serverPort, roomID, bearer, agentBase)

	// CORS middleware (simple, no dependency)
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Origin, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	players := newPlayerStore()
	heartbeatTTL := 10 * time.Second
	initialGrace := 20 * time.Second // bắt đầu kiểm tra sau 20s
	var activeShutdown int32

	// Web UI
	r.GET("/", func(c *gin.Context) {
		html := fmt.Sprintf(ui.ServerUIHTML, roomID, serverPort)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	// API
	r.GET("/heartbeat", func(c *gin.Context) {
		pid := c.Query("player_id")
		if pid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player_id is required"})
			return
		}
		players.heartbeat(pid)
		ginLog("heartbeat from %s", pid)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	r.GET("/players", func(c *gin.Context) {
		list := players.snapshot(time.Now(), heartbeatTTL)
		c.JSON(http.StatusOK, gin.H{"players": list, "room_id": roomID})
	})

	// SDK: build sources and final handler to notify agent and stop server
	sdk := svrsdk.Init(cfg)
	sdk.UseSource(&svrsdk.SignalSource{})
	// Chỉ shutdown khi không còn player nào (size==0) sau grace
	sdk.UseSource(&svrsdk.HeartbeatSource{
		InitialGrace: initialGrace,
		HeartbeatTTL: heartbeatTTL,
		PollInterval: time.Second,
		GetStats: func() (int, bool) {
			now := time.Now()
			active := players.countActive(now, heartbeatTTL)
			return active, active == 0
		},
	})

	srv := &http.Server{Addr: ":" + serverPort, Handler: r}
	go func() {
		ginLog("server listening on :%s room=%s", serverPort, roomID)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if atomic.LoadInt32(&activeShutdown) == 1 {
				ginLog("server closed after active shutdown: %v", err)
				return
			}
			ginLog("server error: %v", err)
			os.Exit(1)
		}
	}()

	shutdownCh := make(chan struct{}, 1)
	sdk.SetFinalHandler(func(sc *svrsdk.ShutdownContext) svrsdk.ShutdownResult {
		// gửi notify tới Agent
		n := &svrsdk.AgentNotifier{}
		err := n.Notify(sc.Ctx, sc.Config, *sc.Event)
		// báo main goroutine để shutdown HTTP server
		atomic.StoreInt32(&activeShutdown, 1)
		select {
		case shutdownCh <- struct{}{}:
		default:
		}
		return svrsdk.ShutdownResult{Decision: svrsdk.DecisionContinue, Err: err}
	})
	stopSources := sdk.Run()
	defer func() {
		if stopSources != nil {
			stopSources()
		}
	}()

	// Giả lập endgame: khi đã có client, sau 5 phút có 50% khả năng kết thúc game
	// go func() {
	// 	rand.Seed(time.Now().UnixNano())
	// 	ticker := time.NewTicker(1 * time.Second)
	// 	defer ticker.Stop()
	// 	// chờ có ít nhất 1 client
	// 	for {
	// 		if atomic.LoadInt32(&activeShutdown) == 1 {
	// 			return
	// 		}
	// 		if players.size() > 0 {
	// 			break
	// 		}
	// 		<-ticker.C
	// 	}
	// 	// đợi 5 phút
	// 	select {
	// 	case <-time.After(5 * time.Minute):
	// 	case <-ticker.C:
	// 	}
	// 	if atomic.LoadInt32(&activeShutdown) == 1 {
	// 		return
	// 	}
	// 	// 50% xác suất
	// 	if rand.Intn(2) == 0 {
	// 		list := players.snapshot(time.Now(), heartbeatTTL)
	// 		if len(list) == 0 {
	// 			return
	// 		}
	// 		scores := map[string]int{}
	// 		winner := ""
	// 		best := -1
	// 		for _, p := range list {
	// 			s := rand.Intn(20)
	// 			scores[p.PlayerID] = s
	// 			if s > best {
	// 				best = s
	// 				winner = p.PlayerID
	// 			}
	// 		}
	// 		_ = sdk.SendShutdownWithDetails(svrsdk.ReasonGameCycleCompleted, map[string]any{
	// 			"winner": winner,
	// 			"scores": scores,
	// 		})
	// 	}
	// }()

	// Chờ sự kiện shutdown từ SDK
	<-shutdownCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	ginLog("game finish")
}

// bytesReader wraps a byte slice into an io.ReadCloser-like Reader for request bodies
// (đã bỏ) helper bytesReader không còn cần thiết vì gửi shutdown dùng SDK
