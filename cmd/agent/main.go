package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"hive/pkg/cron"
	"hive/pkg/mm"
	"hive/pkg/store"
	"hive/pkg/svrmgr"
	"hive/pkg/ui"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/nomad/api"
)

var (
	Version = "dev"
)

func main() {
	gin.SetMode(gin.DebugMode)
	fmt.Fprintf(gin.DefaultWriter, "[GIN-debug] Agent version %s\n", Version)

	// Init subsystems
	storeMgr := store.New("localhost:6379")
	if err := storeMgr.Ping(context.Background()); err != nil {
		log.Fatal("redis not available:", err)
	}
	svrMgr, err := svrmgr.New("http://localhost:4646")
	if err != nil {
		log.Fatal("Failed to create Nomad client:", err)
	}
	mmgr := mm.New(storeMgr, svrMgr)

	// Cron consistency runner
	nc, _ := api.NewClient(&api.Config{Address: "http://localhost:4646"})
	go cron.New(storeMgr, nc, cron.Options{GraceSeconds: 60, JobPrefix: "game-server-", Interval: 10 * time.Second}).Start(context.Background())

	r := gin.Default()

	// Simple web UI
	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(ui.AgentUIHTML))
	})

	// POST/GET create_room: enqueue only
	r.GET("/create_room", func(c *gin.Context) {
		roomID := c.Query("room_id")
		playerID := c.Query("player_id")
		if roomID == "" || playerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "room_id and player_id required"})
			return
		}
		if err := mmgr.CreateRoom(c, roomID, playerID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "room enqueued", "room_id": roomID, "player_id": playerID})
	})

	// GET /join_room: match and launch
	r.GET("/join_room", func(c *gin.Context) {
		playerID := c.Query("player_id")
		if playerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player_id required"})
			return
		}
		st, err := mmgr.JoinRoom(c, playerID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"room_id": st.RoomID, "allocation_id": st.AllocationID, "server_ip": st.ServerIP, "port": st.Port, "players": st.Players})
	})

	// GET /room/:room_id: return current room info
	r.GET("/room/:room_id", func(c *gin.Context) {
		rid := c.Param("room_id")
		if rid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "room_id required"})
			return
		}
		if st, err := storeMgr.GetRoomState(c, rid); err == nil && st != nil && st.ServerIP != "" && st.Port != 0 {
			c.JSON(http.StatusOK, gin.H{"room_id": st.RoomID, "allocation_id": st.AllocationID, "host_ip": st.ServerIP, "ports": gin.H{"http": st.Port}})
			return
		}
		if info, err := svrMgr.GetRoomInfo(rid); err == nil && info != nil {
			c.JSON(http.StatusOK, info)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
	})

	// GET /rooms: list waiting and matched rooms
	r.GET("/rooms", func(c *gin.Context) {
		ctx := c
		pending, err := storeMgr.ListPending(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		roomIDs, err := storeMgr.ListRooms(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		matched := []store.RoomState{}
		for _, id := range roomIDs {
			if st, err := storeMgr.GetRoomState(ctx, id); err == nil && st != nil {
				matched = append(matched, *st)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"waiting": pending,
			"matched": matched,
		})
	})

	// Proxy heartbeat qua Agent để tránh vấn đề mạng/public port
	r.GET("/proxy/heartbeat", func(c *gin.Context) {
		rid := c.Query("room_id")
		pid := c.Query("player_id")
		if rid == "" || pid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "room_id and player_id required"})
			return
		}
		info, err := svrMgr.GetRoomInfo(rid)
		if err != nil || info == nil || info.HostIP == "" || len(info.Ports) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found or not ready"})
			return
		}
		port := 0
		if v, ok := info.Ports["http"]; ok {
			port = v
		} else {
			for _, vv := range info.Ports {
				port = vv
				break
			}
		}
		url := fmt.Sprintf("http://%s:%d/heartbeat?player_id=%s", info.HostIP, port, pid)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			c.Data(resp.StatusCode, "application/json", b)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	log.Fatal(r.Run(":8080"))
}
