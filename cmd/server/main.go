package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

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

func ginLog(format string, args ...interface{}) { fmt.Fprintf(gin.DefaultWriter, format+"\n", args...) }

func main() {
	if len(os.Args) < 2 {
		ginLog("missing port argument")
		os.Exit(1)
	}
	port := os.Args[1]
	if _, err := strconv.Atoi(port); err != nil {
		ginLog("invalid port: %s", port)
		os.Exit(1)
	}
	roomID := ""
	if len(os.Args) >= 3 {
		roomID = os.Args[2]
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	ginLog("startup args: port=%s room=%s", port, roomID)

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
	initialGrace := 10 * time.Second // bắt đầu kiểm tra sau 10s

	// Web UI
	r.GET("/", func(c *gin.Context) {
		html := fmt.Sprintf(ui.ServerUIHTML, roomID, port)
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

	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		ginLog("server listening on :%s room=%s", port, roomID)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ginLog("server error: %v", err)
			os.Exit(1)
		}
	}()

	shutdownCh := make(chan struct{}, 1)
	// Kiểm tra sau initialGrace: nếu chưa có ai heartbeat, shutdown; sau đó theo dõi disconnect > TTL
	go func() {
		time.Sleep(initialGrace)
		if players.size() == 0 {
			ginLog("no players within %s; shutting down", initialGrace)
			shutdownCh <- struct{}{}
			return
		}
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if bad, pid := players.anyDisconnected(time.Now(), heartbeatTTL); bad {
				ginLog("player %s disconnected; shutting down", pid)
				shutdownCh <- struct{}{}
				return
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-shutdownCh:
	case s := <-sigCh:
		ginLog("received %s; shutting down", s.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	ginLog("game finish")
}
