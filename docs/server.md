# Game Server

## Khởi chạy
- Nhận tham số dòng lệnh:
  - arg1: `port` (Nomad dynamic host port)
  - arg2: `room_id`
  - arg3: `bearer_token` (token để gọi Agent shutdown API)
- Ví dụ: `/usr/local/bin/server 31695 flask 1234abcd`

## CORS
- Bật CORS cho mọi `Origin`, cho phép `GET, POST, OPTIONS` và headers cơ bản; trả `204` cho preflight OPTIONS.

## Endpoints
- `GET /heartbeat?player_id=...`: ghi nhận heartbeat, cập nhật `last_seen` người chơi. Endpoint này đồng thời đóng vai trò readiness/liveness probe tối giản ở phía client/agent (double-check readiness: TCP/HTTP).
- `GET /players`: trả `{ players: [{player_id, state, last_seen_unix}], room_id }`
- `GET /`: trang UI hiển thị room, số lượng connected/disconnected, bảng players, log

## Readiness & Shutdown
- **Readiness**: Service sẵn sàng sau khi lắng nghe cổng và trả lời được TCP/HTTP (agent dùng double-check: TCP connect 2 lần trong khoảng ngắn).
- **Initial grace**: Bắt đầu kiểm tra sau `20s` từ khi khởi động.
- **Graceful shutdown conditions**:
  - Không có client heartbeat trong 20s đầu → `no_clients`
  - Client disconnect > `10s` → `client_disconnected`
  - Nhận SIGINT/SIGTERM → `signal_received`
- **Shutdown callback**: Server gửi `POST /rooms/:room_id/shutdown` đến Agent:
  - URL: `http://127.0.0.1:8080/rooms/:room_id/shutdown` (có thể config qua `AGENT_BASE_URL` env)
  - Header: `Authorization: Bearer <token>` (từ arg3)
  - Body: `{ reason: "no_clients|client_disconnected|afk_timeout|game_cycle_completed|signal_received", at?: <unix_ts> }`
  - **Synchronous**: Server đợi callback thành công trước khi shutdown
  - Agent validate token và set room `FULFILLED` với `end_reason`, `graceful_at`, `fulfilled_at`

## UI
- Giao diện nền tối, bảng players, mục log; tự động refresh `/players` mỗi `2s`
