package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the Agent
type Config struct {
	// Server config - HTTP server settings
	Server ServerConfig `json:"server"`

	// Redis config - Cache and state store settings
	Redis RedisConfig `json:"redis"`

	// Nomad config - Job orchestration settings
	Nomad NomadConfig `json:"nomad"`

	// Matchmaking config - Game session management settings
	Matchmaking MatchmakingConfig `json:"matchmaking"`

	// Cron config - Background task settings
	Cron CronConfig `json:"cron"`

	// Timeout config - Various timeout values
	Timeout TimeoutConfig `json:"timeout"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	// Port - HTTP server port to listen on
	// Type: string, Format: "8080", "3000", etc.
	// Range: Valid port numbers (1-65535)
	Port string `json:"port"`
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	// URL - Redis connection string
	// Type: string, Format: "host:port", "localhost:6379"
	// Range: Valid Redis connection strings
	URL string `json:"url"`
}

// NomadConfig holds Nomad job orchestration configuration
type NomadConfig struct {
	// Address - Nomad HTTP API endpoint
	// Type: string, Format: "http://host:port", "http://localhost:4646"
	// Range: Valid HTTP URLs

	// Datacenters - List of Nomad datacenters to deploy jobs to
	// Type: []string, Format: ["dc1"], ["dc1", "dc2"]
	// Range: Valid datacenter names

	// IPMappings - Private to public IP address mappings
	// Type: []IPMapping, Format: [{"private_ip": "172.26.15.163", "public_ip": "52.221.213.97"}]
	// Range: Valid IP address pairs
	Address     string      `json:"address"`
	Datacenters []string    `json:"datacenters"`
	IPMappings  []IPMapping `json:"ip_mappings"`
}

// IPMapping represents a private to public IP address mapping
type IPMapping struct {
	// PrivateIP - Internal/private IP address
	// Type: string, Format: "172.26.15.163", "10.0.0.1"
	// Range: Valid private IP addresses

	// PublicIP - External/public IP address
	// Type: string, Format: "52.221.213.97", "203.0.113.1"
	// Range: Valid public IP addresses
	PrivateIP string `json:"private_ip"`
	PublicIP  string `json:"public_ip"`
}

// MatchmakingConfig holds game session management configuration
type MatchmakingConfig struct {
	// TicketTTL - How long a ticket stays valid before expiring
	// Type: time.Duration, Format: "120s", "2m", "1h"
	// Range: 30s - 1h (recommended: 120s)
	TicketTTL time.Duration `json:"ticket_ttl"`

	// AllocationTimeout - Maximum time to wait for server allocation
	// Type: time.Duration, Format: "2m", "5m", "10m"
	// Range: 1m - 30m (recommended: 2-5m)
	AllocationTimeout time.Duration `json:"allocation_timeout"`

	// AllocationPollDelay - Delay between allocation status checks
	// Type: time.Duration, Format: "2s", "5s", "10s"
	// Range: 1s - 30s (recommended: 2-5s)
	AllocationPollDelay time.Duration `json:"allocation_poll_delay"`
}

// CronConfig holds background task configuration
type CronConfig struct {
	// GraceSeconds - Grace period before cleaning up zombie rooms
	// Type: int64, Format: 60, 120, 300
	// Range: 30 - 600 seconds (recommended: 60-120s)
	GraceSeconds int64 `json:"grace_seconds"`

	// JobPrefix - Prefix for Nomad job names to identify game servers
	// Type: string, Format: "game-server-", "gs-", "game-"
	// Range: Valid string prefixes (recommended: "game-server-")
	JobPrefix string `json:"job_prefix"`

	// Interval - How often to run consistency checks
	// Type: time.Duration, Format: "10s", "30s", "1m"
	// Range: 5s - 5m (recommended: 10-30s)
	Interval time.Duration `json:"interval"`
}

// TimeoutConfig holds various timeout values
type TimeoutConfig struct {
	// HTTPClient - Timeout for HTTP client requests
	// Type: time.Duration, Format: "5s", "10s", "30s"
	// Range: 1s - 2m (recommended: 5-15s)
	HTTPClient time.Duration `json:"http_client"`

	// RedisPing - Timeout for Redis ping operations
	// Type: time.Duration, Format: "2s", "5s", "10s"
	// Range: 1s - 30s (recommended: 2-5s)
	RedisPing time.Duration `json:"redis_ping"`

	// ServerContext - Timeout for server context operations
	// Type: time.Duration, Format: "5s", "10s", "30s"
	// Range: 1s - 2m (recommended: 5-15s)
	ServerContext time.Duration `json:"server_context"`
}

// Default values for all configuration options
// These values are used when environment variables are not set
var defaults = map[string]string{
	// Server Configuration
	"SERVER_PORT": "8080", // Default HTTP server port

	// Redis Configuration
	"REDIS_URL": "localhost:6379", // Default Redis connection string

	// Nomad Configuration
	"NOMAD_ADDRESS":     "http://localhost:4646",       // Default Nomad API endpoint
	"NOMAD_DATACENTERS": "dc1",                         // Default datacenter
	"NOMAD_IP_MAPPINGS": "172.26.15.163:52.221.213.97", // Default IP mapping

	// Matchmaking Configuration
	"TICKET_TTL_SECONDS":            "120", // 2 minutes - ticket validity period
	"ALLOCATION_TIMEOUT_MINUTES":    "2",   // 2 minutes - max wait for server allocation
	"ALLOCATION_POLL_DELAY_SECONDS": "2",   // 2 seconds - delay between allocation checks

	// Cron Configuration
	"CRON_GRACE_SECONDS":    "60",           // 1 minute - grace period before cleanup
	"CRON_JOB_PREFIX":       "game-server-", // Prefix for Nomad job names
	"CRON_INTERVAL_SECONDS": "10",           // 10 seconds - consistency check interval

	// Timeout Configuration
	"HTTP_CLIENT_TIMEOUT_SECONDS":    "5", // 5 seconds - HTTP client timeout
	"REDIS_PING_TIMEOUT_SECONDS":     "2", // 2 seconds - Redis ping timeout
	"SERVER_CONTEXT_TIMEOUT_SECONDS": "5", // 5 seconds - server context timeout
}

// Load creates a new Config with values from environment variables or defaults
func Load() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", defaults["SERVER_PORT"]),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", defaults["REDIS_URL"]),
		},
		Nomad: NomadConfig{
			Address:     getEnv("NOMAD_ADDRESS", defaults["NOMAD_ADDRESS"]),
			Datacenters: getStringSliceEnv("NOMAD_DATACENTERS", defaults["NOMAD_DATACENTERS"]),
			IPMappings:  getIPMappingsEnv("NOMAD_IP_MAPPINGS", defaults["NOMAD_IP_MAPPINGS"]),
		},
		Matchmaking: MatchmakingConfig{
			TicketTTL:           getDurationEnv("TICKET_TTL_SECONDS", defaults["TICKET_TTL_SECONDS"]) * time.Second,
			AllocationTimeout:   getDurationEnv("ALLOCATION_TIMEOUT_MINUTES", defaults["ALLOCATION_TIMEOUT_MINUTES"]) * time.Minute,
			AllocationPollDelay: getDurationEnv("ALLOCATION_POLL_DELAY_SECONDS", defaults["ALLOCATION_POLL_DELAY_SECONDS"]) * time.Second,
		},
		Cron: CronConfig{
			GraceSeconds: getInt64Env("CRON_GRACE_SECONDS", defaults["CRON_GRACE_SECONDS"]),
			JobPrefix:    getEnv("CRON_JOB_PREFIX", defaults["CRON_JOB_PREFIX"]),
			Interval:     getDurationEnv("CRON_INTERVAL_SECONDS", defaults["CRON_INTERVAL_SECONDS"]) * time.Second,
		},
		Timeout: TimeoutConfig{
			HTTPClient:    getDurationEnv("HTTP_CLIENT_TIMEOUT_SECONDS", defaults["HTTP_CLIENT_TIMEOUT_SECONDS"]) * time.Second,
			RedisPing:     getDurationEnv("REDIS_PING_TIMEOUT_SECONDS", defaults["REDIS_PING_TIMEOUT_SECONDS"]) * time.Second,
			ServerContext: getDurationEnv("SERVER_CONTEXT_TIMEOUT_SECONDS", defaults["SERVER_CONTEXT_TIMEOUT_SECONDS"]) * time.Second,
		},
	}

	return cfg
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt64Env(key, defaultValue string) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	if parsed, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
		return parsed
	}
	return 0
}

func getDurationEnv(key, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return time.Duration(parsed)
		}
	}
	if parsed, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
		return time.Duration(parsed)
	}
	return 0
}

func getStringSliceEnv(key, defaultValue string) []string {
	if value := os.Getenv(key); value != "" {
		return []string{value}
	}
	return []string{defaultValue}
}

func getIPMappingsEnv(key, defaultValue string) []IPMapping {
	if value := os.Getenv(key); value != "" {
		// Parse format: "ip1:pub1,ip2:pub2"
		mappings := []IPMapping{}
		pairs := strings.Split(value, ",")
		for _, pair := range pairs {
			parts := strings.Split(pair, ":")
			if len(parts) == 2 {
				mappings = append(mappings, IPMapping{
					PrivateIP: strings.TrimSpace(parts[0]),
					PublicIP:  strings.TrimSpace(parts[1]),
				})
			}
		}
		return mappings
	}

	// Parse default value
	if defaultValue != "" {
		parts := strings.Split(defaultValue, ":")
		if len(parts) == 2 {
			return []IPMapping{{
				PrivateIP: parts[0],
				PublicIP:  parts[1],
			}}
		}
	}
	return []IPMapping{}
}
