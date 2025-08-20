# C# Game Server

C# implementation của Game Server tương tự `cmd/server/main.go` với đầy đủ tính năng graceful shutdown và integration với Agent.

## Tính Năng

- **HTTP Server**: ASP.NET Core với CORS support
- **Player Management**: Quản lý danh sách người chơi và heartbeat
- **Graceful Shutdown**: Tự động shutdown khi không có client hoặc client disconnect
- **Agent Integration**: Gửi shutdown callback đến Agent với token authentication
- **Signal Handling**: Xử lý SIGINT/SIGTERM (Ctrl+C)
- **Logging**: Structured logging với Microsoft.Extensions.Logging

## Cách Sử Dụng

### Build Project
```bash
cd test/cs_server
dotnet build
```

### Chạy Server
```bash
# Cú pháp: cs_server <port> <room_id> [bearer_token]
dotnet run -- 8080 abc123 1234abcd
```

### Biến Môi Trường
```bash
# URL của Agent (mặc định: http://127.0.0.1:8080)
export AGENT_BASE_URL=http://127.0.0.1:8080
```

## API Endpoints

### GET /game/heartbeat?player_id=<player_id>
Ghi nhận heartbeat từ client.

**Response (200 OK):**
```json
{
  "ok": true
}
```

### GET /game/players
Lấy danh sách người chơi và trạng thái.

**Response (200 OK):**
```json
{
  "players": [
    {
      "player_id": "player1",
      "state": "connected",
      "last_seen_unix": 1642531200
    }
  ],
  "room_id": "abc123"
}
```

## Graceful Shutdown

Server tự động shutdown trong các trường hợp:

1. **Không có client**: Sau 20 giây không có heartbeat
2. **Client disconnect**: Client không heartbeat > 10 giây  
3. **Signal nhận**: Ctrl+C hoặc SIGTERM

Khi shutdown, server gửi callback đến Agent:
```
POST {AGENT_BASE_URL}/rooms/{room_id}/shutdown
Authorization: Bearer {bearer_token}
Content-Type: application/json

{
  "reason": "no_clients|client_disconnected|signal_received",
  "at": 1642531200
}
```

## Cấu Trúc Code

### PlayerStore
- Quản lý danh sách người chơi với `ConcurrentDictionary`
- Track `last_seen` timestamp cho mỗi player
- Snapshot với trạng thái connected/disconnected

### GameServer
- Core logic cho monitoring và shutdown
- Gửi shutdown callback đến Agent
- Xử lý signal shutdown

### GameController
- ASP.NET Core controller cho API endpoints
- Heartbeat và players endpoints

## So Sánh với Go Version

| Tính Năng | Go (cmd/server) | C# (test/cs_server) |
|-----------|----------------|-------------------|
| HTTP Server | Gin | ASP.NET Core |
| Player Management | Custom struct | PlayerStore class |
| Graceful Shutdown | ✅ | ✅ |
| Agent Callback | ✅ | ✅ |
| Signal Handling | ✅ | ✅ |
| CORS Support | ✅ | ✅ |
| Logging | fmt.Printf | Microsoft.Extensions.Logging |
| Concurrency | Goroutines | Tasks |

## Testing

### Test với curl
```bash
# Heartbeat
curl "http://localhost:8080/game/heartbeat?player_id=player1"

# Get players
curl "http://localhost:8080/game/players"
```

### Test với Web Client
Server hỗ trợ CORS nên có thể test trực tiếp từ browser.

## Logging

Server ghi log chi tiết:
```
Starting CS Game Server:
  Port: 8080
  Room ID: abc123
  Bearer Token: 1234...
  Agent Base URL: http://127.0.0.1:8080
Server listening on :8080 room=abc123
Heartbeat from player1
No players within 00:00:20; shutting down
Sending shutdown callback: http://127.0.0.1:8080/rooms/abc123/shutdown reason=no_clients
Shutdown callback sent successfully: status=200
Game finish
```
