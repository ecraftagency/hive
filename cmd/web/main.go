package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"hive/pkg/ui"
)

const agentBaseURL = "http://52.221.213.97:8080"

var httpClient = &http.Client{Timeout: 5 * time.Second}

var names = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey", "xray", "yankee", "zulu"}

func randomName() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s-%d", names[rand.Intn(len(names))], rand.Intn(1000))
}

type roomInfo struct {
	RoomID       string         `json:"room_id"`
	AllocationID string         `json:"allocation_id"`
	NodeID       string         `json:"node_id"`
	HostIP       string         `json:"host_ip"`
	Ports        map[string]int `json:"ports"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		poolJSON, _ := json.Marshal(names)
		html := fmt.Sprintf(ui.WebClientHTML, randomName(), string(poolJSON))
		fmt.Fprint(w, html)
	})

	// Proxy APIs to avoid CORS
	mux.HandleFunc("/api/create_room", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		room := r.Form.Get("room_name")
		pid := r.Form.Get("player_id")
		if room == "" || pid == "" {
			http.Error(w, `{"error":"room_name and player_id required"}`, 400)
			return
		}
		url := fmt.Sprintf("%s/create_room?room_id=%s&player_id=%s", agentBaseURL, room, pid)
		proxyJSON(w, r.Context(), url)
	})

	mux.HandleFunc("/api/join_room", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		pid := r.Form.Get("player_id")
		if pid == "" {
			http.Error(w, `{"error":"player_id required"}`, 400)
			return
		}
		url := fmt.Sprintf("%s/join_room?player_id=%s", agentBaseURL, pid)
		proxyJSON(w, r.Context(), url)
	})

	mux.HandleFunc("/api/room_info", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		roomID := r.Form.Get("room_id")
		if roomID == "" {
			http.Error(w, `{"error":"room_id required"}`, 400)
			return
		}
		url := fmt.Sprintf("%s/room/%s", agentBaseURL, roomID)
		proxyJSON(w, r.Context(), url)
	})

	mux.HandleFunc("/api/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		roomID := r.Form.Get("room_id")
		pid := r.Form.Get("player_id")
		if roomID == "" || pid == "" {
			http.Error(w, `{"error":"room_id and player_id required"}`, 400)
			return
		}
		url := fmt.Sprintf("%s/proxy/heartbeat?room_id=%s&player_id=%s", agentBaseURL, roomID, pid)
		proxyJSON(w, r.Context(), url)
	})

	addr := ":8082"
	if v := os.Getenv("WEB_ADDR"); v != "" {
		addr = v
	}
	log.Printf("web client listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func proxyJSON(w http.ResponseWriter, ctx context.Context, url string) {
	resp, err := httpClient.Get(url)
	if err != nil {
		http.Error(w, `{"error":"`+escape(err.Error())+`"}`, 502)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(resp.StatusCode)
		w.Write(b)
		return
	}
	w.Write(b)
}

func escape(s string) string {
	bs, _ := json.Marshal(s)
	var out string
	_ = json.Unmarshal(bs, &out)
	return out
}
