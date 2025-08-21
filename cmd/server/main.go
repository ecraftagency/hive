package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"hive/pkg/ui"

	"bytes"

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
	// Parse command line arguments with new flag structure
	var port, roomID, bearer string

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-port":
			if i+1 < len(os.Args) {
				port = os.Args[i+1]
				i++ // skip next arg
			}
		case "-serverId":
			if i+1 < len(os.Args) {
				roomID = os.Args[i+1]
				i++ // skip next arg
			}
		case "-token":
			if i+1 < len(os.Args) {
				bearer = os.Args[i+1]
				i++ // skip next arg
			}
		}
	}

	// Validate required arguments
	if port == "" {
		ginLog("missing -port argument")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(port); err != nil {
		ginLog("invalid port: %s", port)
		os.Exit(1)
	}
	agentBase := os.Getenv("AGENT_BASE_URL")
	if agentBase == "" {
		agentBase = "http://127.0.0.1:8080"
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	ginLog("startup args: port=%s room=%s bearer=%s agent=%s", port, roomID, bearer, agentBase)

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

	// shutdown callback helper - synchronous để đảm bảo hoàn thành
	sendShutdown := func(reason string) error {
		if roomID == "" || bearer == "" || agentBase == "" {
			ginLog("shutdown callback skipped: roomID=%s bearer=%s agentBase=%s", roomID, bearer, agentBase)
			return fmt.Errorf("missing required parameters")
		}
		payload := map[string]interface{}{"reason": reason, "at": time.Now().Unix()}
		b, _ := json.Marshal(payload)
		url := fmt.Sprintf("%s/rooms/%s/shutdown", agentBase, roomID)
		client := &http.Client{Timeout: 5 * time.Second}
		req, _ := http.NewRequest(http.MethodPost, url, bytesReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bearer)
		ginLog("sending shutdown callback: %s reason=%s", url, reason)
		resp, err := client.Do(req)
		if err != nil {
			ginLog("shutdown callback failed: %v", err)
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			ginLog("shutdown callback failed with status: %d", resp.StatusCode)
			return fmt.Errorf("agent returned status %d", resp.StatusCode)
		}
		ginLog("shutdown callback sent successfully: status=%d", resp.StatusCode)
		return nil
	}

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
			if err := sendShutdown("no_clients"); err != nil {
				ginLog("failed to send shutdown callback: %v", err)
			}
			shutdownCh <- struct{}{}
			return
		}
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if bad, pid := players.anyDisconnected(time.Now(), heartbeatTTL); bad {
				ginLog("player %s disconnected; shutting down", pid)
				if err := sendShutdown("client_disconnected"); err != nil {
					ginLog("failed to send shutdown callback: %v", err)
				}
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
		ginLog("received %s; sending graceful shutdown", s.String())
		if err := sendShutdown("signal_received"); err != nil {
			ginLog("failed to send shutdown callback: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	ginLog("game finish")
}

// bytesReader wraps a byte slice into an io.ReadCloser-like Reader for request bodies
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
