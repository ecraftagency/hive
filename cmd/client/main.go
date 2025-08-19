package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	agentBaseURL = "http://52.221.213.97:8080"
)

type roomInfo struct {
	RoomID       string         `json:"room_id"`
	AllocationID string         `json:"allocation_id"`
	NodeID       string         `json:"node_id"`
	HostIP       string         `json:"host_ip"`
	Ports        map[string]int `json:"ports"`
}

type createRoomResp struct {
	Message  string `json:"message"`
	RoomID   string `json:"room_id"`
	PlayerID string `json:"player_id"`
	Error    string `json:"error"`
}

type joinRoomResp struct {
	RoomID       string   `json:"room_id"`
	AllocationID string   `json:"allocation_id"`
	ServerIP     string   `json:"server_ip"`
	Port         int      `json:"port"`
	Players      []string `json:"players"`
	Error        string   `json:"error"`
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func main() {
	// Generate player ID
	playerID := uuid.New().String()
	fmt.Printf("Player ID: %s\n", playerID)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter command (create_room <room_name> | join_room), quit:")

	for scanner.Scan() {
		args := strings.Fields(scanner.Text())
		if len(args) == 0 {
			fmt.Println("Enter command:")
			continue
		}
		cmd := args[0]

		switch cmd {
		case "create_room":
			if len(args) < 2 {
				fmt.Println("Usage: create_room <room_name>")
				break
			}
			roomName := args[1]
			fmt.Printf("Creating room %s...\n", roomName)
			// Call Agent create_room (enqueue only)
			url := fmt.Sprintf("%s/create_room?room_id=%s&player_id=%s", agentBaseURL, roomName, playerID)
			body, err := httpGet(url)
			if err != nil {
				fmt.Println("Create room failed:", err)
				break
			}
			var cr createRoomResp
			_ = json.Unmarshal(body, &cr)
			if cr.Error != "" {
				fmt.Println("Create room error:", cr.Error)
				break
			}
			fmt.Println("Room enqueued. Ask another client to 'join_room'.")

		case "join_room":
			fmt.Println("Matching... (calling /join_room)")
			for {
				joinURL := fmt.Sprintf("%s/join_room?player_id=%s", agentBaseURL, playerID)
				b, err := httpGet(joinURL)
				if err != nil {
					fmt.Println("join_room failed:", err)
					time.Sleep(2 * time.Second)
					continue
				}
				var jr joinRoomResp
				if err := json.Unmarshal(b, &jr); err != nil {
					fmt.Println("decode join_room failed:", err)
					time.Sleep(2 * time.Second)
					continue
				}
				if jr.Error != "" {
					// likely no pending rooms yet
					fmt.Println("waiting for match:", jr.Error)
					time.Sleep(2 * time.Second)
					continue
				}
				fmt.Printf("Matched room %s\n", jr.RoomID)
				serverIP := jr.ServerIP
				port := jr.Port
				// If agent didn't return server info fully, fallback to polling /room/<room_id>
				if serverIP == "" || port == 0 {
					fmt.Println("Fetching server info from /room...")
					deadline := time.Now().Add(2 * time.Minute)
					for {
						if time.Now().After(deadline) {
							fmt.Println("Timeout waiting for server info")
							break
						}
						infoURL := fmt.Sprintf("%s/room/%s", agentBaseURL, jr.RoomID)
						ib, err := httpGet(infoURL)
						if err == nil {
							var info roomInfo
							if json.Unmarshal(ib, &info) == nil && info.HostIP != "" && len(info.Ports) > 0 {
								serverIP = info.HostIP
								if p, ok := info.Ports["http"]; ok {
									port = p
								} else {
									for _, v := range info.Ports {
										port = v
										break
									}
								}
								break
							}
						}
						time.Sleep(2 * time.Second)
					}
				}
				if serverIP != "" && port != 0 {
					fmt.Printf("Server: %s:%d\n", serverIP, port)
					// Start heartbeat loop every 3s
					hbURL := fmt.Sprintf("http://%s:%d/heartbeat?player_id=%s", serverIP, port, playerID)
					go func() {
						ticker := time.NewTicker(3 * time.Second)
						defer ticker.Stop()
						for range ticker.C {
							if _, err := httpGet(hbURL); err != nil {
								fmt.Println("heartbeat failed:", err)
							}
						}
					}()
				}
				break
			}

		case "quit":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Println("Unknown command. Available: create_room <room_name>, join_room, quit")
		}

		fmt.Println("Enter command:")
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
