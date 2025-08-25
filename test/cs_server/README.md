# C# Game Server - Hive Agent Integration

C# implementation of Game Server fully compliant with the **Hive Agent v2 protocol**. This server implements the exact endpoints, shutdown callbacks, and command line interface expected by the Hive Agent for seamless integration.

## 🚀 **Protocol Compliance Status**

✅ **FULLY COMPLIANT** with Hive Agent v2 protocol
- Command line arguments: `-port <port> -serverId <room_id> -token <bearer_token>`
- API endpoints: `/`, `/heartbeat`, `/players` (direct routes, no `/game` prefix)
- Shutdown reasons: `no_clients`, `client_disconnected`, `afk_timeout`, `game_cycle_completed`, `signal_received`
- Agent callback: `POST /rooms/{room_id}/shutdown` with Bearer token authentication
- Response formats: Exactly match Go server for consistency

## 🎯 **Key Features**

- **HTTP Server**: ASP.NET Core with CORS support
- **Player Management**: Thread-safe player tracking with heartbeat monitoring
- **Graceful Shutdown**: Automatic shutdown based on Agent protocol requirements
- **Agent Integration**: Secure shutdown callbacks with token authentication
- **Signal Handling**: Proper SIGINT/SIGTERM handling for containerized deployments
- **Structured Logging**: Microsoft.Extensions.Logging with detailed operation tracking
- **Protocol Compliance**: 100% compatible with Hive Agent v2 specifications

## 📋 **Command Line Interface**

### **New Format (Agent v2 Compliant)**
```bash
# Required format for Hive Agent integration
dotnet run -- -serverPort <port> -serverId <room_id> -token <bearer_token> -agentUrl <agent_url> [-nographics] [-batchmode]

# Examples:
dotnet run -- -serverPort 8080 -serverId abc123 -token 1234abcd -agentUrl http://localhost:8080
dotnet run -- -serverPort 3000 -serverId room456 -token mysecret -agentUrl https://agent.mycompany.com -nographics -batchmode
dotnet run -- -serverPort 5000 -serverId test789 -agentUrl http://localhost:8080  # Uses default token
```

### **Legacy Format (Deprecated)**
```bash
# ❌ OLD FORMAT - No longer supported
dotnet run -- 8080 abc123 1234abcd  # Positional arguments
dotnet run -- -port 8080 -serverId abc123  # Old -port argument
```

### **Argument Details**
- **`-serverPort`**: Port for HTTP heartbeat server (required)
- **`-serverId`**: Room identifier from Agent (required)
- **`-token`**: Bearer token for Agent authentication (optional, defaults to "1234abcd")
- **`-agentUrl`**: URL of the Agent for shutdown callbacks (required)
- **`-nographics`**: Graphics mode flag (optional)
- **`-batchmode`**: Batch mode flag (optional)

### **HTTP Heartbeat Server**
- **`-serverPort` is required**: Server opens HTTP server on specified port for heartbeat monitoring
- This allows clients to connect and send heartbeat requests for connection tracking

## 🌍 **Environment Variables**

```bash
# Agent base URL for shutdown callbacks
# Default: http://localhost:8080
export AGENT_BASE_URL=http://127.0.0.1:8080

# Custom Agent URL for production
export AGENT_BASE_URL=https://agent.mycompany.com
```

## 🔌 **API Endpoints**

### **GET /** (Root)
Server status and basic information for Agent readiness checks.

**Response (200 OK):**
```json
{
  "room_id": "abc123",
  "connected_players": 2,
  "disconnected_players": 0,
  "total_players": 2,
  "status": "running"
}
```

### **GET /heartbeat?player_id=<player_id>**
Records player heartbeat for connection tracking.

**Parameters:**
- `player_id` (query): Unique player identifier

**Response (200 OK):**
```json
{
  "ok": true
}
```

**Response (400 Bad Request):**
```json
{
  "error": "player_id is required"
}
```

### **GET /players**
Returns current player list and room information.

**Response (200 OK):**
```json
{
  "players": [
    {
      "player_id": "player1",
      "state": "connected",
      "last_seen_unix": 1642531200
    },
    {
      "player_id": "player2", 
      "state": "disconnected",
      "last_seen_unix": 1642531100
    }
  ],
  "room_id": "abc123"
}
```

### **POST /trigger-shutdown** (Development Only)
Triggers signal shutdown for testing purposes.

**Response (200 OK):**
```json
{
  "ok": true,
  "message": "Signal shutdown triggered"
}
```

## 🔄 **Graceful Shutdown Protocol**

The server implements the exact shutdown conditions expected by the Hive Agent:

### **Shutdown Triggers**
1. **No Clients**: After 20 seconds without any player heartbeats
2. **Client Disconnect**: Any player exceeds 10-second heartbeat TTL
3. **Signal Received**: SIGINT (Ctrl+C) or SIGTERM received
4. **AFK Timeout**: Player inactivity beyond configured threshold
5. **Game Cycle Completed**: Game session naturally ended

### **Shutdown Callback**
When shutdown is triggered, the server sends a callback to the Agent:

```
POST {AGENT_BASE_URL}/rooms/{room_id}/shutdown
Authorization: Bearer {bearer_token}
Content-Type: application/json

{
  "reason": "no_clients|client_disconnected|afk_timeout|game_cycle_completed|signal_received",
  "at": 1642531200
}
```

### **Shutdown Reasons**
- **`no_clients`**: No players connected after initial grace period
- **`client_disconnected`**: Player exceeded heartbeat TTL
- **`afk_timeout`**: Player inactive beyond AFK threshold
- **`game_cycle_completed`**: Game session completed naturally
- **`signal_received`**: SIGINT/SIGTERM received

## 🏗️ **Code Architecture**

### **PlayerStore Class**
- **Thread-safe player tracking** using `ConcurrentDictionary`
- **Heartbeat TTL management** for connection status
- **Player snapshot generation** for API responses

### **GameServer Class**
- **Core monitoring logic** implementing Agent protocol
- **Graceful shutdown coordination** across all components
- **Agent communication** with retry logic and error handling

### **GameController Class**
- **Direct API endpoints** (no `/game` prefix) as Agent expects
- **Consistent response formats** matching Go server
- **CORS configuration** for client access

## 🔧 **Build and Deployment**

### **Build Project**
```bash
cd test/cs_server
dotnet build --configuration Release
```

### **Run in Development**
```bash
dotnet run -- -port 8080 -serverId dev123 -token devtoken
```

### **Run in Production**
```bash
# Build release version
dotnet publish --configuration Release --output ./publish

# Run published version
./publish/cs_server -port 8080 -serverId prod456 -token prodsecret
```

### **Docker Deployment**
```dockerfile
FROM mcr.microsoft.com/dotnet/aspnet:7.0
COPY ./publish /app
WORKDIR /app
ENTRYPOINT ["dotnet", "cs_server.dll"]
```

```bash
docker run -p 8080:8080 \
  -e AGENT_BASE_URL=http://agent:8080 \
  cs-server:latest \
  -port 8080 -serverId docker123 -token dockertoken
```

## 🧪 **Testing and Integration**

### **Test with curl**
```bash
# Heartbeat
curl "http://localhost:8080/heartbeat?player_id=player1"

# Get players
curl "http://localhost:8080/players"

# Get server status
curl "http://localhost:8080/"
```

### **Test with Hive Agent**
1. **Start Agent** with proper configuration
2. **Submit ticket** via Agent API
3. **Agent launches** C# server with correct arguments
4. **Server registers** with Agent via shutdown callback
5. **Monitor lifecycle** through Agent UI

### **Integration Checklist**
- [ ] Command line arguments use new flag format
- [ ] Server binds to correct port from arguments
- [ ] Heartbeat endpoint accepts player_id parameter
- [ ] Players endpoint returns correct JSON format
- [ ] Root endpoint provides server status
- [ ] Shutdown callback uses correct Agent endpoint
- [ ] Bearer token authentication works
- [ ] Signal handling (SIGINT/SIGTERM) functional
- [ ] CORS allows client connections
- [ ] Logging provides operational visibility

## 📊 **Monitoring and Observability**

### **Log Output**
```
Starting CS Game Server:
  Port: 8080
  Room ID: abc123
  Bearer Token: 1234...
  Agent Base URL: http://127.0.0.1:8080
  Protocol: Hive Agent v2 (flag-based arguments)
Server listening on :8080 room=abc123
Ready to accept connections from Agent and clients
Heartbeat from player1
No players within 00:00:20; shutting down
Sending shutdown callback: http://127.0.0.1:8080/rooms/abc123/shutdown reason=no_clients
Shutdown callback sent successfully: status=200
Game server shutdown complete
```

### **Health Checks**
- **Readiness**: Server responds to root endpoint
- **Liveness**: Heartbeat endpoint functional
- **Connectivity**: Can reach Agent for callbacks

## 🔒 **Security Considerations**

- **Bearer Token Authentication**: Required for Agent communication
- **Input Validation**: All parameters validated before processing
- **CORS Configuration**: Configured for client access while maintaining security
- **Timeout Handling**: HTTP client timeouts prevent hanging connections

## 🚨 **Troubleshooting**

### **Common Issues**

1. **"player_id is required"**
   - Ensure heartbeat calls include `?player_id=<value>`

2. **"Shutdown callback skipped"**
   - Check `roomId`, `bearerToken`, and `agentBaseUrl` values

3. **"Failed to create HTTP response"**
   - Verify Agent is accessible at configured URL

4. **"Server error: Address already in use"**
   - Port is already bound, use different port

### **Debug Mode**
```bash
# Enable detailed logging
export ASPNETCORE_ENVIRONMENT=Development
dotnet run -- -port 8080 -serverId debug123 -token debug
```

## 📚 **References**

- **Hive Agent Documentation**: See `docs/agent.md`
- **Protocol Specification**: See `docs/arch.md`
- **Integration Guide**: See `docs/integration_guide.md`
- **Go Server Reference**: See `cmd/server/main.go`

## 🤝 **Contributing**

This implementation serves as a reference for third-party developers integrating with the Hive Agent. For questions or improvements:

1. **Review the code** for protocol compliance
2. **Test integration** with your Agent instance
3. **Validate endpoints** match expected behavior
4. **Ensure shutdown callbacks** reach Agent successfully

---

**Note**: This C# server is designed to be a drop-in replacement for the Go server when integrating with the Hive Agent. All protocol requirements have been implemented to ensure seamless operation.
