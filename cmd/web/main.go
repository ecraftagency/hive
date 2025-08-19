package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"hive/pkg/ui"
)

const agentBaseURL = "http://52.221.213.97:8080"

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
		html := fmt.Sprintf(ui.WebClientHTML, randomName(), string(poolJSON), agentBaseURL)
		fmt.Fprint(w, html)
	})

	addr := ":8082"
	if v := os.Getenv("WEB_ADDR"); v != "" {
		addr = v
	}
	log.Printf("web client listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
