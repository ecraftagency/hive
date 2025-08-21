package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"hive/pkg/config"
	"hive/pkg/cron"
	"hive/pkg/dto"
	"hive/pkg/mm"
	"hive/pkg/store"
	"hive/pkg/svrmgr"
	"hive/pkg/ui"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/nomad/api"
)

var (
	Version = "dev"
)

func main() {
	gin.SetMode(gin.DebugMode)
	fmt.Fprintf(gin.DefaultWriter, "[GIN-debug] Agent version %s\n", Version)

	// Load configuration
	cfg := config.Load()
	fmt.Fprintf(gin.DefaultWriter, "[GIN-debug] Config loaded: Redis=%s, Nomad=%s\n", cfg.Redis.URL, cfg.Nomad.Address)

	// Init subsystems
	storeMgr := store.New(cfg.Redis.URL)
	store.SetTicketTTL(cfg.Matchmaking.TicketTTL) // Set TTL from config
	store.SetAllocationTimeout(cfg.Matchmaking.AllocationTimeout)
	store.SetTerminalTTL(cfg.Matchmaking.TerminalTTL)
	if err := storeMgr.Ping(context.Background()); err != nil {
		log.Fatal("redis not available:", err)
	}
	svrMgr, err := svrmgr.New(cfg.Nomad.Address)
	if err != nil {
		log.Fatal("Failed to create Nomad client:", err)
	}
	// Set Nomad configs
	svrMgr.SetDatacenters(cfg.Nomad.Datacenters)
	svrMgr.SetBearerToken(cfg.Auth.BearerToken)
	ipMappings := []svrmgr.IPMapping{}
	for _, mapping := range cfg.Nomad.IPMappings {
		ipMappings = append(ipMappings, svrmgr.IPMapping{
			PrivateIP: mapping.PrivateIP,
			PublicIP:  mapping.PublicIP,
		})
	}
	svrmgr.SetIPMappingConfig(&svrmgr.IPMappingConfig{Mappings: ipMappings})
	mmgr := mm.New(storeMgr, svrMgr)

	// Cron runner
	nc, _ := api.NewClient(&api.Config{Address: cfg.Nomad.Address})
	go cron.New(storeMgr, nc, cron.Options{
		GraceSeconds: cfg.Cron.GraceSeconds,
		JobPrefix:    cfg.Cron.JobPrefix,
		Interval:     cfg.Cron.Interval,
	}).Start(context.Background())

	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://52.221.213.97:8082"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// UI
	r.GET("/", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(ui.AgentUIHTML)) })

	// Admin overview JSON
	r.GET("/admin/overview", func(c *gin.Context) {
		ctx := c
		openTickets, _ := storeMgr.ListOpenedTickets(ctx)
		openedRoomsIDs, _ := storeMgr.ListRooms(ctx) // we only have generic rooms index; load and filter by status
		openedRooms := []store.RoomState{}
		activedRooms := []store.RoomState{}
		fulfilledRooms := []store.RoomState{}
		deadRooms := []store.RoomState{}
		for _, rid := range openedRoomsIDs {
			if st, err := storeMgr.GetRoomState(ctx, rid); err == nil && st != nil {
				switch st.Status {
				case "OPENED":
					openedRooms = append(openedRooms, *st)
				case "ACTIVED":
					activedRooms = append(activedRooms, *st)
				case "FULFILLED":
					fulfilledRooms = append(fulfilledRooms, *st)
				case "DEAD":
					deadRooms = append(deadRooms, *st)
				}
			}
		}
		c.JSON(http.StatusOK, dto.AdminOverviewResponse{
			OpenTickets:    openTickets,
			OpenedRooms:    openedRooms,
			ActivedRooms:   activedRooms,
			FulfilledRooms: fulfilledRooms,
			DeadRooms:      deadRooms,
		})
	})

	// Matchmaking APIs
	// Submit ticket
	r.POST("/tickets", func(c *gin.Context) {
		var req dto.SubmitTicketRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				ErrorCode: dto.ErrCodeMissingPlayerID,
				Error:     "player_id required",
			})
			return
		}
		t, err := mmgr.SubmitJoinTicket(c, req.PlayerID)
		if err != nil {
			c.JSON(http.StatusOK, dto.SubmitTicketResponse{Status: "REJECTED"})
			return
		}
		c.JSON(http.StatusOK, dto.SubmitTicketResponse{
			TicketID: t.TicketID,
			Status:   t.Status,
		})
		// best-effort: trigger a simple matcher (optional)
		go func() { _, _ = mmgr.TryMatch(context.Background()) }()
	})

	// Ticket status
	r.GET("/tickets/:id", func(c *gin.Context) {
		id := c.Param("id")
		t, err := mmgr.GetTicket(c, id)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				ErrorCode: dto.ErrCodeTicketNotFound,
				Error:     "ticket not found",
			})
			return
		}
		c.JSON(http.StatusOK, dto.TicketStatusResponse{
			Status: t.Status,
			RoomID: t.RoomID,
		})
	})

	// Cancel ticket
	r.POST("/tickets/:id/cancel", func(c *gin.Context) {
		id := c.Param("id")
		if err := mmgr.CancelTicket(c, id); err != nil {
			c.JSON(http.StatusBadRequest, dto.ToErrorResponse(err))
			return
		}
		c.JSON(http.StatusOK, dto.CancelTicketResponse{Status: "CANCELED"})
	})

	// --- Room APIs ---
	r.GET("/rooms/:room_id", func(c *gin.Context) {
		rid := c.Param("room_id")
		if rid == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				ErrorCode: dto.ErrCodeMissingRoomID,
				Error:     "room_id required",
			})
			return
		}
		if st, err := storeMgr.GetRoomState(c, rid); err == nil && st != nil {
			c.JSON(http.StatusOK, st)
			return
		}
		if info, err := svrMgr.GetRoomInfo(rid); err == nil && info != nil {
			c.JSON(http.StatusOK, info)
			return
		}
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			ErrorCode: dto.ErrCodeRoomNotFound,
			Error:     "room not found",
		})
	})

	// Shutdown callback từ server → Agent
	r.POST("/rooms/:room_id/shutdown", func(c *gin.Context) {
		rid := c.Param("room_id")
		if rid == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{ErrorCode: dto.ErrCodeMissingRoomID, Error: "room_id required"})
			return
		}
		// Xác thực token Authorization: Bearer <token>
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{ErrorCode: dto.ErrCodeUnauthorized, Error: "missing authorization header"})
			return
		}
		expectedToken := "Bearer " + cfg.Auth.BearerToken
		if authHeader != expectedToken {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{ErrorCode: dto.ErrCodeUnauthorized, Error: "invalid authorization token"})
			return
		}
		var body dto.ShutdownRequest
		if err := c.BindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{ErrorCode: dto.ErrCodeInvalidRequest, Error: fmt.Sprintf("invalid shutdown request: %v", err)})
			return
		}
		// Validate reason là một trong những giá trị hợp lệ
		validReasons := map[string]bool{
			"no_clients":           true,
			"client_disconnected":  true,
			"afk_timeout":          true,
			"game_cycle_completed": true,
			"signal_received":      true,
		}
		if !validReasons[body.Reason] {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{ErrorCode: dto.ErrCodeInvalidRequest, Error: fmt.Sprintf("invalid reason: %s. Valid reasons: no_clients, client_disconnected, afk_timeout, game_cycle_completed, signal_received", body.Reason)})
			return
		}
		st, err := storeMgr.GetRoomState(c, rid)
		if err != nil || st == nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{ErrorCode: dto.ErrCodeRoomNotFound, Error: "room not found"})
			return
		}
		if st.Status != "ACTIVED" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{ErrorCode: dto.ErrCodeInvalidRequest, Error: fmt.Sprintf("room status is %s, not ACTIVED", st.Status)})
			return
		}
		if body.At == 0 {
			body.At = time.Now().Unix()
		}
		// set FULFILLED với end_reason và graceful_at
		st.Status = "FULFILLED"
		st.EndReason = body.Reason
		st.FulfilledAt = body.At
		st.GracefulAt = body.At
		// Details (winner/scores) nếu có
		if body.Details != nil {
			if v, ok := body.Details["winner"].(string); ok {
				st.Winner = v
			}
			if m, ok := body.Details["scores"].(map[string]interface{}); ok {
				st.Scores = map[string]int{}
				for k, vv := range m {
					switch n := vv.(type) {
					case float64:
						st.Scores[k] = int(n)
					case int:
						st.Scores[k] = n
					}
				}
			}
		}
		_ = storeMgr.SaveRoomState(c, *st)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// legacy rooms list (giữ tạm)
	r.GET("/rooms", func(c *gin.Context) {
		ctx := c
		roomIDs, err := storeMgr.ListRooms(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ToErrorResponse(err))
			return
		}
		matched := []store.RoomState{}
		for _, id := range roomIDs {
			if st, err := storeMgr.GetRoomState(ctx, id); err == nil && st != nil {
				matched = append(matched, *st)
			}
		}
		c.JSON(http.StatusOK, gin.H{"matched": matched})
	})

	// --- Legacy proxy (giữ nếu cần UI cũ) ---
	r.GET("/proxy/heartbeat", func(c *gin.Context) {
		rid := c.Query("room_id")
		pid := c.Query("player_id")
		if rid == "" || pid == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				ErrorCode: dto.ErrCodeInvalidRequest,
				Error:     "room_id and player_id required",
			})
			return
		}
		info, err := svrMgr.GetRoomInfo(rid)
		if err != nil || info == nil || info.HostIP == "" || len(info.Ports) == 0 {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				ErrorCode: dto.ErrCodeRoomNotReady,
				Error:     "room not found or not ready",
			})
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
		client := &http.Client{Timeout: cfg.Timeout.HTTPClient}
		resp, err := client.Get(url)
		if err != nil {
			c.JSON(http.StatusBadGateway, dto.ErrorResponse{
				ErrorCode: dto.ErrCodeGatewayError,
				Error:     err.Error(),
			})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			c.Data(resp.StatusCode, "application/json", b)
			return
		}
		c.JSON(http.StatusOK, dto.ProxyHeartbeatResponse{OK: true})
	})

	log.Fatal(r.Run(":" + cfg.Server.Port))
}
