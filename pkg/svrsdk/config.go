package svrsdk

import (
	"os"
)

// FromEnvOrArgs ưu tiên ENV rồi fallback flags -port, -serverId, -token, -agentBase
func FromEnvOrArgs(args []string) Config {
	cfg := Config{
		Port:         os.Getenv("HIVE_PORT"),
		RoomID:       os.Getenv("HIVE_ROOM_ID"),
		Token:        os.Getenv("HIVE_TOKEN"),
		AgentBaseURL: os.Getenv("HIVE_AGENT_BASE_URL"),
	}
	// Backward-compatible fallback
	if cfg.AgentBaseURL == "" {
		cfg.AgentBaseURL = os.Getenv("AGENT_BASE_URL")
	}

	// Fallback flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-port":
			if i+1 < len(args) {
				cfg.Port = args[i+1]
				i++
			}
		case "-serverId":
			if i+1 < len(args) {
				cfg.RoomID = args[i+1]
				i++
			}
		case "-token":
			if i+1 < len(args) {
				cfg.Token = args[i+1]
				i++
			}
		case "-agentBase":
			if i+1 < len(args) {
				cfg.AgentBaseURL = args[i+1]
				i++
			}
		case "-nographics":
			cfg.NoGraphics = true
		case "-batchmode":
			cfg.BatchMode = true
		}
	}

	if cfg.AgentBaseURL == "" {
		cfg.AgentBaseURL = "http://127.0.0.1:8080"
	}
	if cfg.Token == "" {
		cfg.Token = "1234abcd"
	}
	return cfg
}
