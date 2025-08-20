package main

import (
	"fmt"
	"log"
	"net/http"

	"hive/pkg/config"
	"hive/pkg/svrmgr"

	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var (
	Version = "dev"
)

func main() {
	gin.SetMode(gin.DebugMode)
	fmt.Fprintf(gin.DefaultWriter, "[GIN-debug] Agent v2 version %s\n", Version)

	// Load configuration
	cfg := config.Load()

	// Ensure Nomad address has fallback
	nomadAddress := cfg.Nomad.Address
	if nomadAddress == "" {
		nomadAddress = "http://localhost:4646"
	}
	fmt.Fprintf(gin.DefaultWriter, "[GIN-debug] Config loaded: Nomad=%s\n", nomadAddress)

	// Init server manager
	svrMgr, err := svrmgr.New(nomadAddress)
	if err != nil {
		log.Fatal("Failed to create Nomad client:", err)
	}

	// Set Nomad configs with fallbacks
	datacenters := cfg.Nomad.Datacenters
	if len(datacenters) == 0 {
		datacenters = []string{"dc1"}
	}
	svrMgr.SetDatacenters(datacenters)

	// Set IP mappings with fallback
	ipMappings := []svrmgr.IPMapping{}
	if len(cfg.Nomad.IPMappings) > 0 {
		for _, mapping := range cfg.Nomad.IPMappings {
			ipMappings = append(ipMappings, svrmgr.IPMapping{
				PrivateIP: mapping.PrivateIP,
				PublicIP:  mapping.PublicIP,
			})
		}
	} else {
		// Fallback to default IP mapping
		ipMappings = []svrmgr.IPMapping{
			{PrivateIP: "10.0.0.23", PublicIP: "20.205.180.232"},
		}
	}
	svrmgr.SetIPMappingConfig(&svrmgr.IPMappingConfig{Mappings: ipMappings})

	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Create room endpoint
	r.GET("/create_room", func(c *gin.Context) {
		// Cap số lượng job game-server đang chạy ở mức 4
		running, err := svrMgr.CountRunningJobsByNamePrefix("game-server-")
		if err == nil && running >= 4 {
			c.JSON(http.StatusOK, gin.H{
				"status":  "fail",
				"message": "capacity reached",
				"running": running,
				"limit":   4,
			})
			return
		}

		roomID := c.Query("room_id")
		if roomID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "room_id required in query params",
			})
			return
		}

		// Hardcoded resource values
		cpu := 2048          // 2048 MHz
		memoryMB := 4096 * 2 // 2048 MB

		// Parse game_type from query param
		gameType := c.Query("game_type")
		if gameType == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "game_type required in query params",
			})
			return
		}

		// Map game_type -> numeric gametype value
		mappedGameType := ""
		switch gameType {
		case "waitingBattle_BattleRoyale_Queue_product":
			mappedGameType = "0"
		case "waitingBattle_BattleRoyale_Queue_product_rank":
			mappedGameType = "1"
		case "waitingBattle_BattleRoyale_Queue_product_custom":
			mappedGameType = "2"
		case "testqueue":
			mappedGameType = "3"
		default:
			log.Printf("invalid game_type received: %s", gameType)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid game_type",
				"details": gameType,
			})
			return
		}

		// Build arguments with session_id and mapped gametype
		args := []string{"-session=" + roomID, "-gametype=" + mappedGameType}

		// Start Nomad job with room_id and custom resources
		command := "/usr/local/bin/linuxserver/MetaDOSServer.x86_64"
		if err := svrMgr.RunGameServerV2(roomID, cpu, memoryMB, command, args); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to start server: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":           "success",
			"room_id":          roomID,
			"game_type":        gameType,
			"gametype_numeric": mappedGameType,
			"cpu":              cpu,
			"memory_mb":        memoryMB,
			"args":             args,
			"message":          "Server job started successfully",
		})
	})

	// Room status endpoint for polling
	r.GET("/room/:id", func(c *gin.Context) {
		roomID := c.Param("id")
		if roomID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "room_id required",
			})
			return
		}

		// Try get room info
		roomInfo, err := svrMgr.GetRoomInfo(roomID)
		if err != nil {
			// Not allocated yet → allocating/expired depends on age
			// We don't have direct created_at fetch, so we fallback: treating as allocating (<60s) else expired
			// In a real setup, should fetch job meta created_at
			age := 0 * time.Second
			status := "allocating"
			if age >= 60*time.Second {
				status = "expired"
			}
			if status == "expired" {
				c.JSON(http.StatusOK, gin.H{
					"room_id": roomID,
					"status":  "expired",
					"message": fmt.Sprintf("Room not allocated in 60s: %v", err),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"room_id": roomID,
				"status":  "allocating",
				"message": "Server is allocating",
			})
			return
		}

		// Allocated?
		if roomInfo.HostIP != "" && len(roomInfo.Ports) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"room_id": roomID,
				"status":  "allocated",
				"host_ip": roomInfo.HostIP,
				"ports":   roomInfo.Ports,
				"message": "Server is allocated",
			})
			return
		}

		// Otherwise still allocating
		c.JSON(http.StatusOK, gin.H{
			"room_id": roomID,
			"status":  "allocating",
			"message": "Server is allocating",
		})
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": Version,
		})
	})

	log.Fatal(r.Run(":8080"))
}
