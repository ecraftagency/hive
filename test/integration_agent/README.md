# Integration Agent

Test agent để kiểm tra server behavior theo protocol đã định nghĩa.

## Cấu trúc Project

```
integration_agent/
├── integration_agent.csproj
├── Program.cs                 # Main entry point
├── TestConfig.cs             # Configuration
├── LogEntry.cs               # Log entry structure
├── ServerInfo.cs             # Server information
├── LogMonitor.cs             # Monitor server logs
├── ServerLauncher.cs         # Launch server process
├── ProtocolValidator.cs      # Validate server behavior
├── appsettings.json          # Default configuration
└── Scenarios/
    ├── ITestScenario.cs      # Interface for test scenarios
    └── NoClientsScenario.cs  # No clients graceful shutdown test
```

## Features

- ✅ Launch server với đầy đủ arguments và environment variables
- ✅ Monitor stdout/stderr của server process
- ✅ Pattern matching cho server events
- ✅ Validate graceful shutdown timing và reason
- ✅ Support `-nographics` và `-batchmode` flags
- ✅ Tránh port conflict với real agent (8080)

## Server Arguments

Server được launch với:
```bash
./server -port <port> -serverId <room_id> -token <bearer_token> -nographics -batchmode
```

Environment variables:
```bash
AGENT_BASE_URL=http://localhost:8081
```

## Test Scenarios

### 1. No Clients Graceful Shutdown
- Launch server
- Wait 20s cho graceful shutdown
- Validate shutdown signal với reason="no_clients"
- Validate timing trong khoảng 18-22s

## Usage

```bash
# Build project
dotnet build

# Run với default config
dotnet run

# Run với custom server path
dotnet run --server-path ./server.exe

# Run với custom config
dotnet run --agent-port 8082 --server-path ./server.exe
```

## Configuration

File `appsettings.json`:
```json
{
  "TestConfig": {
    "ServerPath": "./server",
    "ServerToken": "test-token",
    "UseNoGraphics": true,
    "UseBatchMode": true,
    "AgentPort": 8081,
    "AgentBaseUrl": "http://localhost:8081",
    "GraceShutdownTimeout": "00:00:20",
    "MinShutdownTime": "00:00:18",
    "MaxShutdownTime": "00:00:22"
  }
}
```

## Next Steps

1. ✅ Basic infrastructure
2. ✅ NoClientsScenario
3. 🔄 ClientDisconnectScenario (cần implement)
4. 🔄 EndGameScenario (cần implement)
5. 🔄 HTTP endpoint để nhận shutdown signals
6. 🔄 Client simulator cho heartbeat testing

## Build Status

Hiện tại có một số lỗi build cần fix:
- Missing using statements
- TcpListener compatibility
- Configuration binding

Cần hoàn thiện để có thể chạy được.
