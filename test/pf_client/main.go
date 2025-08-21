package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	TitleID        = "FA0D0"
	BaseURL        = "https://FA0D0.playfabapi.com"
	AgentV2BaseURL = "http://20.205.180.232:8080"
)

// PlayFab API models
type LoginWithCustomIDRequest struct {
	TitleID       string `json:"TitleId"`
	CustomID      string `json:"CustomId"`
	CreateAccount bool   `json:"CreateAccount"`
}

type LoginWithCustomIDResult struct {
	Code   int       `json:"code"`
	Status string    `json:"status"`
	Data   LoginData `json:"data"`
}

type LoginData struct {
	PlayFabID     string      `json:"PlayFabId"`
	SessionTicket string      `json:"SessionTicket"`
	EntityToken   EntityToken `json:"EntityToken"`
}

type EntityToken struct {
	EntityToken     string `json:"EntityToken"`
	TokenExpiration string `json:"TokenExpiration"`
	Entity          Entity `json:"Entity"`
}

type CreateMatchmakingTicketRequest struct {
	Creator            CreatorEntity `json:"Creator"`
	GiveUpAfterSeconds int           `json:"GiveUpAfterSeconds"`
	QueueName          string        `json:"QueueName"`
}

type CreatorEntity struct {
	Entity Entity `json:"Entity"`
}

type Entity struct {
	ID   string `json:"Id"`
	Type string `json:"Type"`
}

type CreateMatchmakingTicketResult struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
	Data   struct {
		TicketID string `json:"TicketId"`
	} `json:"data"`
}

type GetMatchmakingTicketRequest struct {
	TicketID  string `json:"TicketId"`
	QueueName string `json:"QueueName"`
}

type GetMatchmakingTicketResult struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
	Data   struct {
		TicketID string              `json:"TicketId"`
		Status   string              `json:"Status"`
		MatchID  string              `json:"MatchId,omitempty"`
		Members  []MatchmakingPlayer `json:"Members,omitempty"`
	} `json:"data"`
}

type MatchmakingPlayer struct {
	Attributes MatchmakingPlayerAttributes `json:"Attributes"`
	Entity     Entity                      `json:"Entity"`
}

type MatchmakingPlayerAttributes struct {
	DataObject map[string]interface{} `json:"DataObject"`
}

// Agent v2 room response
type AgentRoomResponse struct {
	RoomID  string         `json:"room_id"`
	Status  string         `json:"status"`
	HostIP  string         `json:"host_ip"`
	Ports   map[string]int `json:"ports"`
	Message string         `json:"message"`
}

type PlayFabClient struct {
	HTTPClient    *http.Client
	SessionTicket string
	EntityToken   string
	EntityID      string
}

func NewPlayFabClient() *PlayFabClient {
	return &PlayFabClient{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *PlayFabClient) LoginWithCustomID(customID string) error {
	req := LoginWithCustomIDRequest{
		TitleID:       TitleID,
		CustomID:      customID,
		CreateAccount: true,
	}

	reqBody, _ := json.Marshal(req)
	fmt.Printf("🔐 Login request: %s\n", string(reqBody))

	resp, err := c.HTTPClient.Post(BaseURL+"/Client/LoginWithCustomID", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("📡 Login response status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		// Read error response body
		errorBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status: %d, body: %s", resp.StatusCode, string(errorBody))
	}

	// Read raw response body first
	rawBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("📋 Raw response body: %s\n", string(rawBody))

	var result LoginWithCustomIDResult
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return fmt.Errorf("failed to decode login response: %v", err)
	}

	fmt.Printf("📋 Parsed response: %+v\n", result)

	c.SessionTicket = result.Data.SessionTicket
	c.EntityToken = result.Data.EntityToken.EntityToken
	c.EntityID = result.Data.EntityToken.Entity.ID

	if c.SessionTicket == "" {
		return fmt.Errorf("session ticket is empty")
	}

	fmt.Printf("✅ Logged in successfully with CustomID: %s, PlayFabID: %s\n", customID, result.Data.PlayFabID)
	return nil
}

func (c *PlayFabClient) CreateMatchmakingTicket(queueName string) (string, error) {
	req := CreateMatchmakingTicketRequest{
		Creator: CreatorEntity{
			Entity: Entity{
				ID:   c.EntityID, // Use Entity ID from login response
				Type: "title_player_account",
			},
		},
		GiveUpAfterSeconds: 60,
		QueueName:          queueName,
	}

	reqBody, _ := json.Marshal(req)
	fmt.Printf("🎫 Create ticket request: %s\n", string(reqBody))
	httpReq, _ := http.NewRequest("POST", BaseURL+"/Match/CreateMatchmakingTicket", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Authorization", c.SessionTicket)
	httpReq.Header.Set("X-EntityToken", c.EntityToken)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("create ticket request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read error response body
		errorBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create ticket failed with status: %d, body: %s", resp.StatusCode, string(errorBody))
	}

	// Read raw response body first
	rawBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("📋 Create ticket raw response: %s\n", string(rawBody))

	var result CreateMatchmakingTicketResult
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode create ticket response: %v", err)
	}

	fmt.Printf("🎫 Created matchmaking ticket: %s\n", result.Data.TicketID)
	return result.Data.TicketID, nil
}

func (c *PlayFabClient) GetMatchmakingTicket(ticketID, queueName string) (*GetMatchmakingTicketResult, error) {
	req := GetMatchmakingTicketRequest{
		TicketID:  ticketID,
		QueueName: queueName,
	}

	reqBody, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", BaseURL+"/Match/GetMatchmakingTicket", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Authorization", c.SessionTicket)
	httpReq.Header.Set("X-EntityToken", c.EntityToken)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get ticket request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get ticket failed with status: %d", resp.StatusCode)
	}

	// Read raw response body first
	rawBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("📋 Get ticket raw response: %s\n", string(rawBody))

	var result GetMatchmakingTicketResult
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode get ticket response: %v", err)
	}

	return &result, nil
}

func (c *PlayFabClient) PollMatchmakingTicket(ticketID, queueName string) (*GetMatchmakingTicketResult, error) {
	fmt.Printf("🔍 Polling ticket %s...\n", ticketID)

	for i := 0; i < 30; i++ { // Poll for up to 30 seconds
		result, err := c.GetMatchmakingTicket(ticketID, queueName)
		if err != nil {
			return nil, err
		}

		fmt.Printf("   Status: %s", result.Data.Status)
		if result.Data.MatchID != "" {
			fmt.Printf(", Match ID: %s", result.Data.MatchID)
		}
		if len(result.Data.Members) > 0 {
			fmt.Printf(", Players: %d", len(result.Data.Members))
		}
		fmt.Println()

		switch result.Data.Status {
		case "Matched":
			fmt.Printf("🎯 Match found! Match ID: %s\n", result.Data.MatchID)
			return result, nil
		case "Canceled":
			return result, fmt.Errorf("ticket was canceled")
		case "Expired":
			return result, fmt.Errorf("ticket expired")
		}

		time.Sleep(6 * time.Second)
	}

	return nil, fmt.Errorf("polling timeout")
}

// Poll agent_v2 room status until ready and then print phase to connect
func PollAgentRoomReady(matchID string) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	fmt.Printf("🔄 Polling agent_v2 for room %s...\n", matchID)
	for i := 0; i < 40; i++ { // ~4 minutes with 6s interval
		url := fmt.Sprintf("%s/room/%s", AgentV2BaseURL, matchID)
		resp, err := httpClient.Get(url)
		if err != nil {
			fmt.Printf("   agent_v2 request error: %v\n", err)
			time.Sleep(6 * time.Second)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Handle non-200
			var apiErr struct {
				Error     string `json:"error"`
				ErrorCode string `json:"error_code"`
			}
			_ = json.Unmarshal(raw, &apiErr)
			if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
				msg := apiErr.Error
				if msg == "" {
					msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
				return fmt.Errorf("agent_v2 error: %s", msg)
			}
			// Transient/server errors -> retry
			fmt.Printf("   agent_v2 non-OK (%d), will retry...\n", resp.StatusCode)
			time.Sleep(6 * time.Second)
			continue
		}

		var room AgentRoomResponse
		if err := json.Unmarshal(raw, &room); err != nil {
			fmt.Printf("   agent_v2 decode error: %v\n", err)
			time.Sleep(6 * time.Second)
			continue
		}
		fmt.Printf("   agent_v2 status: %s\n", room.Status)
		switch room.Status {
		case "allocating":
			// continue polling
			time.Sleep(6 * time.Second)
			continue
		case "expired":
			return fmt.Errorf("allocation expired")
		case "allocated":
			fmt.Printf("phase: connect to photon session %s\n", matchID)
			return nil
		default:
			// backward compat for previous agent_v2
			if room.Status == "ready" && room.HostIP != "" {
				fmt.Printf("phase: connect to photon session %s\n", matchID)
				return nil
			}
			// otherwise, wait
			time.Sleep(6 * time.Second)
		}
	}
	return fmt.Errorf("agent_v2 room not ready in time")
}

func main() {
	fmt.Println("🚀 PlayFab Matchmaking Client Starting...")
	fmt.Printf("📋 Title ID: %s\n", TitleID)
	fmt.Printf("🌐 Base URL: %s\n", BaseURL)
	fmt.Println()

	// Create two clients for two players
	player1 := NewPlayFabClient()
	player2 := NewPlayFabClient()

	// Login both players
	fmt.Println("🔐 Logging in players...")
	if err := player1.LoginWithCustomID("player1"); err != nil {
		log.Fatalf("Player 1 login failed: %v", err)
	}
	if err := player2.LoginWithCustomID("player2"); err != nil {
		log.Fatalf("Player 2 login failed: %v", err)
	}
	fmt.Println()

	// Submit matchmaking tickets
	fmt.Println("🎫 Submitting matchmaking tickets...")
	ticket1, err := player1.CreateMatchmakingTicket("testqueue")
	if err != nil {
		log.Fatalf("Player 1 ticket creation failed: %v", err)
	}

	ticket2, err := player2.CreateMatchmakingTicket("testqueue")
	if err != nil {
		log.Fatalf("Player 2 ticket creation failed: %v", err)
	}
	fmt.Println()

	// Poll for match results
	fmt.Println("⏳ Waiting for match...")

	// Start polling in goroutines
	done := make(chan bool, 2)

	go func() {
		result, err := player1.PollMatchmakingTicket(ticket1, "testqueue")
		if err != nil {
			fmt.Printf("❌ Player 1 polling failed: %v\n", err)
		} else {
			fmt.Printf("✅ Player 1 match result: %+v\n", result)
			// After matched, poll agent_v2 for room readiness
			if result != nil && result.Data.MatchID != "" {
				if err := PollAgentRoomReady(result.Data.MatchID); err != nil {
					fmt.Printf("❌ Player 1 agent_v2 polling failed: %v\n", err)
				}
			}
		}
		done <- true
	}()

	go func() {
		result, err := player2.PollMatchmakingTicket(ticket2, "testqueue")
		if err != nil {
			fmt.Printf("❌ Player 2 polling failed: %v\n", err)
		} else {
			fmt.Printf("✅ Player 2 match result: %+v\n", result)
			// After matched, poll agent_v2 for room readiness
			if result != nil && result.Data.MatchID != "" {
				if err := PollAgentRoomReady(result.Data.MatchID); err != nil {
					fmt.Printf("❌ Player 2 agent_v2 polling failed: %v\n", err)
				}
			}
		}
		done <- true
	}()

	// Wait for both players to complete
	<-done
	<-done

	fmt.Println("\n🎉 Matchmaking simulation completed!")
}
