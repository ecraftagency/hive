# Hướng Dẫn Tích Hợp Agent

Tài liệu tích hợp cho bên thứ 3 với Hive Agent.

## 1. API Endpoints

### Base URL
```
http://<agent-host>:8080
```

### POST /tickets
**Request:**
```json
{"player_id": "string"}
```

**Response:**
```json
{"ticket_id": "uuid", "status": "OPENED"}
```

**Error:**
- `400`: `{"error_code": "MISSING_PLAYER_ID"}`
- `200`: `{"status": "REJECTED"}` (duplicate player)

### GET /tickets/{ticket_id}
**Response:**
```json
{"status": "OPENED|MATCHED|EXPIRED|REJECTED", "room_id": "uuid"}
```

**Error:**
- `404`: `{"error_code": "TICKET_NOT_FOUND"}`

### POST /tickets/{ticket_id}/cancel
**Response:**
```json
{"status": "CANCELED"}
```

### GET /rooms/{room_id}
**Response:**
```json
{
  "status": "OPENED|ACTIVED|DEAD|FULFILLED",
  "server_ip": "string",
  "port": 8080,
  "players": ["player1", "player2"],
  "fail_reason": "string", // DEAD only
  "end_reason": "string"   // FULFILLED only
}
```

## 2. Client Integration Flow

### Step 1: Submit Ticket
```javascript
const response = await fetch('/tickets', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({player_id: 'player1'})
});

if (response.status === 200) {
  const data = await response.json();
  if (data.status === 'REJECTED') {
    // Handle duplicate player
    return;
  }
  const ticketId = data.ticket_id;
  // Continue to polling
}
```

### Step 2: Poll Ticket Status
```javascript
async function pollTicket(ticketId) {
  while (true) {
    const response = await fetch(`/tickets/${ticketId}`);
    const data = await response.json();
    
    switch (data.status) {
      case 'OPENED':
        await sleep(2000); // Wait 2s
        break;
      case 'MATCHED':
        return data.room_id; // Success
      case 'EXPIRED':
      case 'REJECTED':
        throw new Error(`Ticket ${data.status}`);
    }
  }
}
```

### Step 3: Poll Room Status
```javascript
async function pollRoom(roomId) {
  while (true) {
    const response = await fetch(`/rooms/${roomId}`);
    const data = await response.json();
    
    switch (data.status) {
      case 'OPENED':
        await sleep(2000);
        break;
      case 'ACTIVED':
        return {ip: data.server_ip, port: data.port}; // Success
      case 'DEAD':
        throw new Error(`Room dead: ${data.fail_reason}`);
      case 'FULFILLED':
        throw new Error(`Room fulfilled: ${data.end_reason}`);
    }
  }
}
```

### Step 4: Connect to Game Server
```javascript
const serverInfo = await pollRoom(roomId);
const gameUrl = `http://${serverInfo.ip}:${serverInfo.port}`;

// Start heartbeat
setInterval(async () => {
  try {
    await fetch(`${gameUrl}/heartbeat?player_id=player1`);
  } catch (error) {
    console.error('Heartbeat failed:', error);
  }
}, 3000);
```

## 3. Error Handling

### Network Errors
```javascript
async function apiCall(url, options) {
  try {
    const response = await fetch(url, options);
    if (!response.ok) {
      const error = await response.json();
      throw new Error(`${response.status}: ${error.error_code}`);
    }
    return await response.json();
  } catch (error) {
    if (error.name === 'TypeError') {
      // Network error - retry with backoff
      await sleep(1000);
      return apiCall(url, options);
    }
    throw error;
  }
}
```

### Timeout Handling
```javascript
const TIMEOUTS = {
  TICKET_TTL: 120000,      // 2 minutes
  ALLOCATION: 120000,      // 2 minutes
  POLL_INTERVAL: 2000,     // 2 seconds
  HEARTBEAT: 3000          // 3 seconds
};

// Set timeout for each step
setTimeout(() => {
  // Handle timeout
}, TIMEOUTS.TICKET_TTL);
```

### Status Code Mapping
| Status | Action |
|--------|--------|
| `400` | Retry with valid data |
| `404` | Resource not found, stop polling |
| `500` | Server error, retry with backoff |
| `502` | Gateway error, retry with backoff |

## 4. Game Server Integration

### Server Startup
```bash
# Command line flags (new)
./server -port <port> -serverId <room_id> -token <bearer_token> -nographics -batchmode

# Environment
AGENT_BASE_URL=http://127.0.0.1:8080
```

### Graceful Shutdown
```javascript
// Send shutdown callback to Agent
const response = await fetch(`${AGENT_BASE_URL}/rooms/${roomId}/shutdown`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${bearerToken}`
  },
  body: JSON.stringify({
    reason: 'no_clients|client_disconnected|signal_received',
    at: Date.now() / 1000
  })
});
```

### Shutdown Reasons
- `no_clients`: No heartbeat after 20s
- `client_disconnected`: Client timeout > 10s
- `signal_received`: SIGINT/SIGTERM
- `afk_timeout`: Client AFK (future)
- `game_cycle_completed`: Game finished (future)

## 5. Configuration

### Environment Variables
```bash
# Agent
AGENT_BASE_URL=http://127.0.0.1:8080
AGENT_BEARER_TOKEN=1234abcd

# Timeouts
TICKET_TTL_SECONDS=120
ALLOCATION_TIMEOUT_MINUTES=2
TERMINAL_TTL_SECONDS=60
```

### Polling Strategy
```javascript
const POLLING_CONFIG = {
  ticket: {interval: 2000, timeout: 120000},
  room: {interval: 2000, timeout: 120000},
  heartbeat: {interval: 3000, timeout: 10000}
};
```

## 6. Testing Checklist

- [ ] Submit ticket → get ticket_id
- [ ] Poll ticket → MATCHED with room_id
- [ ] Poll room → ACTIVED with server info
- [ ] Connect to game server
- [ ] Send heartbeat every 3s
- [ ] Handle network errors
- [ ] Handle timeout scenarios
- [ ] Test graceful shutdown
- [ ] Verify Agent callback

## 7. Common Issues

### Ticket Rejected
- Check for duplicate player_id
- Wait before retry

### Room Dead
- Check allocation timeout
- Verify Nomad cluster status

### Network Errors
- Implement exponential backoff
- Add retry mechanism

### Timeout
- Increase timeout values
- Check server load
