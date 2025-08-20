# Hướng Dẫn Tích Hợp Agent

Tài liệu này cung cấp thông tin chi tiết để tích hợp với dịch vụ Hive Agent, bao gồm tài liệu API, luồng ticket và chi tiết cấu hình.

---

## PHẦN 1: Tài Liệu API Agent

### Base URL
```
http://<agent-host>:<port>
```

### Xác Thực
Hiện tại không yêu cầu xác thực. Tất cả các endpoint đều có thể truy cập công khai.

### CORS
Agent hỗ trợ CORS cho web client. Các origin được cấu hình có thể thiết lập qua biến môi trường.

---

### Các Endpoint

#### 1. Gửi Ticket (Tham Gia Phòng)
**POST** `/tickets`

Tạo ticket mới cho người chơi để tham gia phòng game.

**Request Body:**
```json
{
  "player_id": "string"
}
```

**Các Trường Request:**
- `player_id` (bắt buộc): Định danh duy nhất cho người chơi. Phải là chuỗi không rỗng.

**Response (200 OK):**
```json
{
  "ticket_id": "uuid-string",
  "status": "OPENED"
}
```

**Các Trường Response:**
- `ticket_id`: Định danh ticket duy nhất (định dạng UUID)
- `status`: Trạng thái ticket hiện tại (luôn là "OPENED" ban đầu)

**Response (200 OK - Bị Từ Chối):**
```json
{
  "status": "REJECTED"
}
```

**Response Lỗi:**
- `400 Bad Request`: `{"error_code": "MISSING_PLAYER_ID", "error": "player_id required"}`

**Logic Nghiệp Vụ:**
- Ngăn chặn ticket trùng lặp cho cùng một người chơi
- Tự động kích hoạt quá trình matchmaking
- Ticket vào hàng đợi chờ để ghép cặp

---

#### 2. Lấy Trạng Thái Ticket
**GET** `/tickets/{ticket_id}`

Lấy trạng thái hiện tại của ticket.

**Tham Số Đường Dẫn:**
- `ticket_id`: UUID của ticket cần kiểm tra

**Response (200 OK):**
```json
{
  "status": "OPENED|MATCHED|EXPIRED|REJECTED|CANCELED",
  "room_id": "uuid-string"
}
```

**Các Trường Response:**
- `status`: Trạng thái ticket hiện tại
  - `OPENED`: Ticket đang chờ ghép cặp
  - `MATCHED`: Ticket đã được ghép cặp với người chơi khác
  - `EXPIRED`: Ticket hết hạn mà không có ghép cặp (sau TTL)
  - `REJECTED`: Ticket bị từ chối (người chơi trùng lặp, v.v.)
  - `CANCELED`: Ticket bị hủy thủ công
- `room_id`: Định danh phòng (chỉ có khi status là "MATCHED")

**Response Lỗi:**
- `404 Not Found`: `{"error_code": "TICKET_NOT_FOUND", "error": "ticket not found"}`

---

#### 3. Hủy Ticket
**POST** `/tickets/{ticket_id}/cancel`

Hủy ticket đang mở và xóa khỏi hàng đợi.

**Tham Số Đường Dẫn:**
- `ticket_id`: UUID của ticket cần hủy

**Request Body:**
```json
{}
```

**Response (200 OK):**
```json
{
  "status": "CANCELED"
}
```

**Response Lỗi:**
- `400 Bad Request`: `{"error_code": "TICKET_CANCEL_FAILED", "error": "error message"}`

**Logic Nghiệp Vụ:**
- Chỉ hoạt động với ticket có trạng thái "OPENED"
- Xóa ticket khỏi hàng đợi chờ
- Cho phép người chơi gửi ticket mới

---

#### 4. Lấy Thông Tin Phòng
**GET** `/rooms/{room_id}`

Lấy thông tin chi tiết về một phòng cụ thể.

**Tham Số Đường Dẫn:**
- `room_id`: UUID của phòng

**Response (200 OK) - Trạng Thái Phòng Từ Redis:**
```json
{
  "room_id": "uuid-string",
  "allocation_id": "uuid-string",
  "server_ip": "52.221.213.97",
  "port": 26793,
  "players": ["player1", "player2"],
  "created_at_unix": 1755620466,
  "status": "OPENED|ACTIVED|DEAD|FULFILLED",
  "fail_reason": "alloc_timeout|nomad_error|server_crash",
  "end_reason": "no_clients|client_disconnected|afk_timeout|game_cycle_completed"
}
```

**Response (200 OK) - Thông Tin Phòng Từ Nomad:**
```json
{
  "room_id": "uuid-string",
  "allocation_id": "uuid-string",
  "node_id": "uuid-string",
  "host_ip": "172.26.15.163",
  "ports": {
    "http": 26793
  }
}
```

**Các Trường Response:**
- `room_id`: Định danh phòng duy nhất
- `allocation_id`: ID allocation của Nomad
- `server_ip`/`host_ip`: Địa chỉ IP server (IP công khai nếu được map)
- `port`/`ports`: Cổng server
- `players`: Mảng các ID người chơi trong phòng
- `created_at_unix`: Timestamp Unix khi phòng được tạo
- `status`: Trạng thái phòng (chỉ có trong trạng thái Redis)
  - `OPENED`: Vừa tạo, đang allocate
  - `ACTIVED`: Allocate ok và readiness đạt (double-check pass)
  - `DEAD`: Allocate thất bại/timeout (terminal, TTL ngắn)
  - `FULFILLED`: Kết thúc vòng chơi/graceful shutdown (terminal, TTL ngắn)
- `fail_reason`: Lý do thất bại khi `DEAD`
- `node_id`: ID node Nomad (chỉ có trong thông tin Nomad)

**Response Lỗi:**
- `400 Bad Request`: `{"error_code": "MISSING_ROOM_ID", "error": "room_id required"}`
- `404 Not Found`: `{"error_code": "ROOM_NOT_FOUND", "error": "room not found"}`

---

#### 5. Tổng Quan Admin
**GET** `/admin/overview`

Lấy tổng quan toàn diện về tất cả trạng thái hệ thống (chỉ admin).

**Response (200 OK):**
```json
{
  "open_tickets": [
    {
      "ticket_id": "uuid-string",
      "player_id": "string",
      "enqueue_at_unix": 1755620466,
      "status": "OPENED"
    }
  ],
  "opened_rooms": [
    {
      "room_id": "uuid-string",
      "players": ["player1", "player2"],
      "created_at_unix": 1755620466,
      "status": "OPENED"
    }
  ],
  "actived_rooms": [
    {
      "room_id": "uuid-string",
      "allocation_id": "uuid-string",
      "server_ip": "52.221.213.97",
      "port": 26793,
      "players": ["player1", "player2"],
      "created_at_unix": 1755620466,
      "status": "ACTIVED"
    }
  ],
  "fulfilled_rooms": [
    {
      "room_id": "uuid-string",
      "allocation_id": "uuid-string",
      "server_ip": "52.221.213.97",
      "port": 26793,
      "players": ["player1", "player2"],
      "created_at_unix": 1755620466,
      "status": "FULFILLED"
    }
  ],
  "dead_rooms": [
    {
      "room_id": "uuid-string",
      "players": ["player1", "player2"],
      "fail_reason": "alloc_timeout|nomad_error|server_crash",
      "created_at_unix": 1755620466,
      "status": "DEAD"
    }
  ]
}
```

---

#### 6. Giao Diện Web
**GET** `/`

Phục vụ giao diện web dashboard của Agent hiển thị trạng thái hệ thống thời gian thực.

---

### Định Dạng Response Lỗi
Tất cả response lỗi đều theo định dạng này:
```json
{
  "error_code": "ERROR_CODE_CONSTANT",
  "error": "Human readable error message"
}
```

**Các Mã Lỗi Thường Gặp:**
- `MISSING_PLAYER_ID`: Thiếu trường player_id bắt buộc
- `MISSING_ROOM_ID`: Thiếu tham số room_id bắt buộc
- `INVALID_REQUEST`: Định dạng request không hợp lệ
- `TICKET_NOT_FOUND`: Ticket được chỉ định không tồn tại
- `ROOM_NOT_FOUND`: Phòng được chỉ định không tồn tại
- `ROOM_NOT_READY`: Phòng tồn tại nhưng server chưa sẵn sàng
- `TICKET_REJECTED`: Ticket bị từ chối bởi logic nghiệp vụ
- `TICKET_CANCEL_FAILED`: Không thể hủy ticket
- `INTERNAL_ERROR`: Lỗi server không mong đợi
- `REDIS_ERROR`: Thao tác Redis thất bại
- `NOMAD_ERROR`: Thao tác Nomad thất bại
- `GATEWAY_ERROR`: Thao tác Gateway/proxy thất bại

---

## PHẦN 2: Luồng Gửi Ticket và Polling (cập nhật)

### Tổng Quan
Hệ thống ticket triển khai state machine để quản lý yêu cầu matchmaking của người chơi. Người chơi gửi ticket, được ghép cặp với người chơi khác, sau đó phòng được cấp phát với game server.

### Sơ Đồ Luồng
```
Người chơi gửi ticket → Ticket OPENED → Matchmaking → Phòng OPENED → Allocate Nomad (double-check) → ACTIVED | DEAD → FULFILLED
     ↓                    ↓              ↓              ↓                    ↓             ↓             ↓
  Trả về Ticket ID    Chờ hàng đợi   Tìm ghép cặp   room_id(UUID)     Server ready?   Terminal fail   End cycle
                                                         (polling)      (Redis/Nomad)   (TTL ~60s)     (TTL ~60s)
```

### Luồng Chi Tiết

#### Giai Đoạn 1: Gửi Ticket
1. **Client gọi** `POST /tickets` với `player_id`
2. **Agent xác thực** request và kiểm tra người chơi trùng lặp
3. **Nếu hợp lệ**: Tạo ticket với trạng thái "OPENED", trả về `ticket_id`
4. **Nếu trùng lặp**: Trả về trạng thái "REJECTED"
5. **Agent kích hoạt** quá trình matchmaking bất đồng bộ

#### Giai Đoạn 2: Matchmaking
1. **Agent kiểm tra** hàng đợi ticket chờ
2. **Nếu có 2+ ticket**: Ghép cặp chúng lại với nhau
3. **Tạo** `room_id` duy nhất (UUID)
4. **Cập nhật** cả hai ticket thành trạng thái "MATCHED" với `room_id`
5. **Tạo** phòng với trạng thái "OPENED"
6. **Bắt đầu** quá trình cấp phát server bất đồng bộ

#### Giai Đoạn 3: Cấp Phát Server (double-check)
1. **Agent gửi** Nomad job `game-server-{room_id}`
2. **Job chạy** với cấp phát cổng động
3. **Double-check** readiness: (1) Allocation RUNNING/healthy, (2) readiness (TCP/HTTP) sau một khoảng ngắn
4. **Khi sẵn sàng**: Cập nhật phòng với IP/port server, trạng thái "ACTIVED"
5. **Nếu timeout**: Đánh dấu phòng là "DEAD" với lý do thất bại
6. **Khi kết thúc vòng chơi**: Server gửi `POST /rooms/:room_id/shutdown` với `Authorization: Bearer <token>` và `reason` để Agent set `FULFILLED` (terminal, TTL)

#### Giai Đoạn 4: Client Polling
1. **Client poll** `GET /tickets/{ticket_id}` cho đến khi trạng thái "MATCHED"
2. **Khi ghép cặp**: Client nhận được `room_id`
3. **Client poll** `GET /rooms/{room_id}` cho đến khi server sẵn sàng
4. **Khi sẵn sàng**: Client bắt đầu heartbeat đến game server
5. **Game bắt đầu**: Người chơi có thể kết nối đến game server

### Chiến Lược Polling

#### Polling Trạng Thái Ticket
- **Tần suất**: Mỗi 2-3 giây
- **Tiếp tục cho đến khi**: Trạng thái thay đổi từ "OPENED"
- **Thành công**: Trạng thái trở thành "MATCHED" với `room_id`
- **Thất bại**: Trạng thái trở thành "EXPIRED" hoặc "REJECTED"

#### Polling Trạng Thái Phòng
- **Tần suất**: Mỗi 2-3 giây
- **Tiếp tục cho đến khi**: `ACTIVED` (Redis) hoặc có `host_ip+port` (Nomad)
- **Thành công**: `ACTIVED` với `server_ip/port` hoặc fallback `host_ip/port`
- **Thất bại**: Trạng thái phòng `DEAD` với `fail_reason`
 - **Kết thúc**: Trạng thái `FULFILLED` khi server graceful shutdown với `end_reason`, client dừng kết nối đến server

### Timeout và Giới Hạn
- **Ticket TTL**: 120 giây (có thể cấu hình)
- **Allocation timeout**: 90 giây (có thể cấu hình)
- **Poll delay**: 2 giây (có thể cấu hình)
- **Terminal TTL**: `DEAD`/`FULFILLED` lưu 60 giây (có thể cấu hình)
- **Grace period**: 60 giây để dọn dẹp `OPENED` quá hạn (có thể cấu hình)

### Xử Lý Lỗi
- **Lỗi mạng**: Thử lại với exponential backoff
- **Lỗi server**: Ghi log và tiếp tục polling
- **Lỗi nghiệp vụ**: Dừng polling và hiển thị thông báo lỗi
- **Timeout**: Xử lý nhẹ nhàng với thông báo cho người dùng

---

## PHẦN 3: Chi Tiết Cấu Hình Agent

### Cấu Trúc Cấu Hình
Cấu hình Agent được tổ chức thành các phần logic, mỗi phần xử lý các khía cạnh cụ thể của hệ thống:

```
Config
├── Server (cài đặt HTTP server)
├── Redis (Cache và lưu trữ trạng thái)
├── Nomad (Điều phối job)
├── Matchmaking (Quản lý phiên game)
├── Cron (Tác vụ nền)
└── Timeout (Các timeout khác nhau)
```

### Biến Môi Trường

#### Cấu Hình Server
```bash
# Cổng HTTP server để lắng nghe
# Loại: string, Định dạng: "8080", "3000", v.v.
# Phạm vi: Số cổng hợp lệ (1-65535)
# Mặc định: 8080
SERVER_PORT=8080
```

#### Cấu Hình Redis
```bash
# Chuỗi kết nối Redis
# Loại: string, Định dạng: "host:port", "localhost:6379"
# Phạm vi: Chuỗi kết nối Redis hợp lệ
# Mặc định: localhost:6379
REDIS_URL=localhost:6379
```

#### Cấu Hình Nomad
```bash
# Endpoint HTTP API của Nomad
# Loại: string, Định dạng: "http://host:port", "http://localhost:4646"
# Phạm vi: URL HTTP hợp lệ
# Mặc định: http://localhost:4646
NOMAD_ADDRESS=http://localhost:4646

# Danh sách datacenter của Nomad để triển khai job
# Loại: string, Định dạng: "dc1", "dc1,dc2"
# Phạm vi: Tên datacenter hợp lệ
# Mặc định: dc1
NOMAD_DATACENTERS=dc1

# Map địa chỉ IP private sang public
# Loại: string, Định dạng: "private1:public1,private2:public2"
# Phạm vi: Cặp địa chỉ IP hợp lệ
# Mặc định: 172.26.15.163:52.221.213.97
NOMAD_IP_MAPPINGS=172.26.15.163:52.221.213.97
```

#### Cấu Hình Matchmaking
```bash
# Thời gian ticket có hiệu lực trước khi hết hạn (tính bằng giây)
# Loại: integer, Định dạng: 120, 300, 600
# Phạm vi: 30 - 3600 giây (30s - 1h)
# Mặc định: 120 (2 phút)
TICKET_TTL_SECONDS=120

# Thời gian tối đa chờ cấp phát server (tính bằng phút)
# Loại: integer, Định dạng: 2, 5, 10
# Phạm vi: 1 - 30 phút
# Mặc định: 2 phút
ALLOCATION_TIMEOUT_MINUTES=2

# Độ trễ giữa các lần kiểm tra trạng thái allocation (tính bằng giây)
# Loại: integer, Định dạng: 2, 5, 10
# Phạm vi: 1 - 30 giây
# Mặc định: 2 giây
ALLOCATION_POLL_DELAY_SECONDS=2
```

#### Cấu Hình Cron
```bash
# Thời gian grace period trước khi dọn dẹp zombie room (tính bằng giây)
# Loại: integer, Định dạng: 60, 120, 300
# Phạm vi: 30 - 600 giây (30s - 10m)
# Mặc định: 60 (1 phút)
CRON_GRACE_SECONDS=60

# Prefix cho tên job Nomad để xác định game server
# Loại: string, Định dạng: "game-server-", "gs-", "game-"
# Phạm vi: Prefix chuỗi hợp lệ
# Mặc định: game-server-
CRON_JOB_PREFIX=game-server-

# Tần suất chạy kiểm tra tính nhất quán (tính bằng giây)
# Loại: integer, Định dạng: 10, 30, 60
# Phạm vi: 5 - 300 giây (5s - 5m)
# Mặc định: 10 giây
CRON_INTERVAL_SECONDS=10
```

#### Cấu Hình Timeout
```bash
# Timeout cho request HTTP client (tính bằng giây)
# Loại: integer, Định dạng: 5, 10, 30
# Phạm vi: 1 - 120 giây (1s - 2m)
# Mặc định: 5 giây
HTTP_CLIENT_TIMEOUT_SECONDS=5

# Timeout cho thao tác ping Redis (tính bằng giây)
# Loại: integer, Định dạng: 2, 5, 10
# Phạm vi: 1 - 30 giây (1s - 30s)
# Mặc định: 2 giây
REDIS_PING_TIMEOUT_SECONDS=2

# Timeout cho thao tác context server (tính bằng giây)
# Loại: integer, Định dạng: 5, 10, 30
# Phạm vi: 1 - 120 giây (1s - 2m)
# Mặc định: 5 giây
SERVER_CONTEXT_TIMEOUT_SECONDS=5
```

### Tải Cấu Hình
1. **Biến môi trường** được kiểm tra trước tiên
2. **Giá trị mặc định** được sử dụng nếu env vars không được thiết lập
3. **Chuyển đổi kiểu** được xử lý tự động (string → int64, time.Duration)
4. **Xác thực** đảm bảo giá trị nằm trong phạm vi chấp nhận được

### Ví Dụ Cấu Hình Production
```bash
# Môi trường production
SERVER_PORT=8080
REDIS_URL=redis-cluster.example.com:6379
NOMAD_ADDRESS=https://nomad.example.com:4646
NOMAD_DATACENTERS=prod-dc1,prod-dc2
NOMAD_IP_MAPPINGS=10.0.1.100:203.0.113.10,10.0.1.101:203.0.113.11

# Timeout mạnh mẽ cho production
TICKET_TTL_SECONDS=300
ALLOCATION_TIMEOUT_MINUTES=5
ALLOCATION_POLL_DELAY_SECONDS=1

# Kiểm tra tính nhất quán thường xuyên
CRON_GRACE_SECONDS=30
CRON_INTERVAL_SECONDS=5

# Timeout bảo thủ
HTTP_CLIENT_TIMEOUT_SECONDS=10
REDIS_PING_TIMEOUT_SECONDS=5
SERVER_CONTEXT_TIMEOUT_SECONDS=10
```

### Xác Thực Cấu Hình
- **Số cổng**: Phải nằm trong phạm vi cổng hợp lệ (1-65535)
- **URL**: Phải là URL HTTP/HTTPS hợp lệ
- **Timeout**: Phải là giá trị dương trong phạm vi hợp lý
- **Địa chỉ IP**: Phải là địa chỉ IPv4 hoặc IPv6 hợp lệ
- **Datacenter**: Phải là tên datacenter Nomad hợp lệ

### Hot Reload
Cấu hình được tải khi khởi động. Để thay đổi cấu hình:
1. **Cập nhật** biến môi trường
2. **Khởi động lại** dịch vụ Agent
3. **Xác minh** giá trị mới trong log

### Giám Sát Cấu Hình
- **Log level**: Thiết lập qua biến môi trường `GIN_MODE`
- **Metrics**: Có sẵn qua endpoint `/admin/overview`
- **Health checks**: Kết nối Redis và trạng thái API Nomad
- **Hiệu suất**: Giám sát thời gian phản hồi và tỷ lệ lỗi

---

## PHẦN 4: Tích Hợp Game Server

### Tổng Quan
Game Server được khởi chạy bởi Agent thông qua Nomad job và cần tích hợp với Agent để thực hiện graceful shutdown.

### Khởi Chạy Game Server

#### Tham Số Dòng Lệnh
Game Server nhận 3 tham số từ Nomad job:
```bash
/usr/local/bin/server <port> <room_id> <bearer_token>
```

**Các Tham Số:**
- `port`: Cổng động được cấp phát bởi Nomad
- `room_id`: Định danh phòng (UUID)
- `bearer_token`: Token để xác thực với Agent

#### Biến Môi Trường
```bash
# URL của Agent để gửi shutdown callback
# Mặc định: http://127.0.0.1:8080
AGENT_BASE_URL=http://127.0.0.1:8080
```

### API Endpoints

#### 1. Heartbeat Endpoint
**GET** `/heartbeat?player_id=<player_id>`

Ghi nhận heartbeat từ client.

**Tham Số Query:**
- `player_id`: Định danh người chơi

**Response (200 OK):**
```json
{
  "ok": true
}
```

**Response Lỗi:**
- `400 Bad Request`: `{"error": "player_id is required"}`

#### 2. Players Endpoint
**GET** `/players`

Lấy danh sách người chơi và trạng thái.

**Response (200 OK):**
```json
{
  "players": [
    {
      "player_id": "string",
      "state": "connected|disconnected",
      "last_seen_unix": 1642531200
    }
  ],
  "room_id": "uuid-string"
}
```

### Graceful Shutdown Flow

#### Điều Kiện Shutdown
Game Server tự động shutdown trong các trường hợp:

1. **Không có client**: Sau 20 giây không có heartbeat
2. **Client disconnect**: Client không heartbeat > 10 giây
3. **Signal nhận**: SIGINT/SIGTERM từ hệ thống

#### Shutdown Callback
Khi phát hiện điều kiện shutdown, server gửi callback đến Agent:

**POST** `{AGENT_BASE_URL}/rooms/{room_id}/shutdown`

**Headers:**
```
Content-Type: application/json
Authorization: Bearer <bearer_token>
```

**Request Body:**
```json
{
  "reason": "no_clients|client_disconnected|afk_timeout|game_cycle_completed|signal_received",
  "at": 1642531200
}
```

**Các Reason:**
- `no_clients`: Không có client kết nối sau 20s
- `client_disconnected`: Client disconnect > 10s
- `afk_timeout`: Client AFK quá lâu (chưa implement)
- `game_cycle_completed`: Hoàn thành vòng chơi (chưa implement)
- `signal_received`: Nhận SIGINT/SIGTERM

**Response (200 OK):**
```json
{
  "ok": true,
  "status": "FULFILLED"
}
```

**Response Lỗi:**
- `401 Unauthorized`: Token không hợp lệ
- `400 Bad Request`: Room không ở trạng thái ACTIVED
- `404 Not Found`: Room không tồn tại

#### Synchronous Shutdown
- Server đợi callback thành công trước khi shutdown
- Timeout 5 giây cho callback
- Log chi tiết kết quả callback

### CORS Configuration
Game Server hỗ trợ CORS cho web client:

```go
// Headers được hỗ trợ
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true
Access-Control-Allow-Methods: GET,POST,OPTIONS
Access-Control-Allow-Headers: Content-Type, Origin, Accept, Authorization
```

### Web UI
**GET** `/`

Hiển thị giao diện quản lý phòng:
- Thông tin phòng và cổng
- Số lượng client connected/disconnected
- Bảng danh sách người chơi
- Log hoạt động

### Logging
Game Server ghi log các sự kiện quan trọng:

```
startup args: port=31695 room=abc123 token_present=true
server listening on :31695 room=abc123
heartbeat from player1
no players within 20s; shutting down
sending shutdown callback: http://127.0.0.1:8080/rooms/abc123/shutdown reason=no_clients
shutdown callback sent successfully: status=200
game finish
```

### Xử Lý Lỗi

#### Callback Failures
- **Network error**: Log lỗi và tiếp tục shutdown
- **Invalid token**: Log lỗi xác thực
- **Room not found**: Log lỗi trạng thái phòng
- **Timeout**: Log timeout và tiếp tục shutdown

#### Client Connection Issues
- **Heartbeat timeout**: Tự động disconnect client
- **Invalid player_id**: Trả lỗi 400
- **Server overload**: Có thể reject heartbeat

### Monitoring & Health Checks

#### Health Check Endpoints
- **Readiness**: `GET /players` (trả 200 khi server sẵn sàng)
- **Liveness**: TCP connect đến cổng server

#### Metrics
- Số lượng client connected
- Tần suất heartbeat
- Thời gian uptime
- Số lần shutdown

### Best Practices

#### Development
1. **Test locally**: Chạy server với tham số thủ công
2. **Mock Agent**: Sử dụng mock server cho testing
3. **Log levels**: Bật debug log cho development

#### Production
1. **Resource limits**: Cấu hình memory/CPU limits
2. **Graceful shutdown**: Đảm bảo callback luôn được gửi
3. **Monitoring**: Giám sát uptime và error rates
4. **Backup strategy**: Có plan B nếu Agent không available

#### Security
1. **Token validation**: Đảm bảo bearer token hợp lệ
2. **Input validation**: Validate tất cả input từ client
3. **Rate limiting**: Giới hạn tần suất heartbeat
4. **Network security**: Sử dụng HTTPS cho callback

---

## Danh Sách Kiểm Tra Tích Hợp

### Trước Khi Tích Hợp
- [ ] Xem xét tài liệu API và mã lỗi
- [ ] Hiểu luồng ticket và chiến lược polling
- [ ] Cấu hình biến môi trường
- [ ] Test với môi trường development

### Trong Quá Trình Tích Hợp
- [ ] Triển khai xử lý lỗi phù hợp
- [ ] Sử dụng khoảng thời gian polling thích hợp
- [ ] Xử lý timeout một cách nhẹ nhàng
- [ ] Ghi log tất cả tương tác API

### Sau Khi Tích Hợp
- [ ] Giám sát tỷ lệ lỗi và thời gian phản hồi
- [ ] Xác minh hoàn thành vòng đời ticket
- [ ] Test cấp phát server và kết nối game
- [ ] Xác thực dọn dẹp và kiểm tra tính nhất quán

### Xử Lý Sự Cố
- **Tỷ lệ lỗi cao**: Kiểm tra kết nối mạng và timeout
- **Cấp phát chậm**: Xem xét dung lượng cluster Nomad và cấu hình job
- **Vấn đề bộ nhớ**: Giám sát sử dụng Redis và connection pooling
- **Hiệu suất**: Điều chỉnh khoảng thời gian polling và giá trị timeout

---

*Cập nhật lần cuối: 2025-01-19*
*Phiên bản: 1.1*
