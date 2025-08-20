# Agent

## Nhiệm vụ (cập nhật)
- Nhận ticket tham gia (join) từ client chỉ với `player_id` (không nhập `room_id`).
- Ghép phòng (match) → khi match found, Agent tự sinh `room_id` (UUID) và tạo room OPENED.
- Khởi chạy Nomad job `game-server-<room_id>`; cập nhật state room qua double-check allocate: chỉ khi Nomad RUNNING/healthy và readiness của room service đạt mới chuyển `ACTIVED` (ghi server ip/port, allocation_id vào Redis).
- Cung cấp API truy vấn ticket/room; cron đồng bộ Redis ↔ Nomad: `OPENED` quá hạn → `DEAD(alloc_timeout)`; `ACTIVED` mà job dừng/xóa nhưng không có tín hiệu graceful → `DEAD(server_crash)`; `FULFILLED` chỉ được set khi agent nhận tín hiệu graceful từ server (terminal, TTL ngắn).
- UI `/ui`: hiển thị Waiting/Matched/Actived, auto-refresh.

## Lưu trữ (tóm tắt)
- Tickets: `mm:ticket:<id>` (TTL 120s), `mm:tickets:opened`, `mm:players:pending`
- Rooms: `mm:room:opened:<room_id>`, `mm:room:actived:<room_id>`, `mm:room:dead:<room_id>`, `mm:room:fulfilled:<room_id>`
- Index: `mm:rooms:opened`, `mm:rooms:actived`, `mm:rooms:dead`, `mm:rooms:fulfilled`

## API (matchmaking mới)
- `POST /tickets`
  - Body: `{ player_id }`
  - Validate: `player_id` không có ticket OPENED → nếu vi phạm trả `REJECTED`
  - Response: `{ ticket_id, status: "OPENED"|"REJECTED" }`
- `GET /tickets/:ticket_id`
  - Response: `{ status: "OPENED"|"MATCHED"|"EXPIRED"|"REJECTED", room_id? }`
- `POST /tickets/:ticket_id/cancel`
  - Chỉ khi ticket đang `OPENED`; xóa khỏi queue/index → `{ status: "CANCELED" }`
- `GET /rooms/:room_id`
  - Response: `{ status: "OPENED"|"ACTIVED"|"DEAD"|"FULFILLED", server?, fail_reason?, players }` (luôn 200; trong TTL terminal không trả 404)

### API Shutdown (server → agent)
- `POST /rooms/:room_id/shutdown`
  - Header: `Authorization: Bearer <token>` (token từ `AGENT_BEARER_TOKEN` env, truyền cho server qua args)
  - Body: `{ reason: "no_clients|client_disconnected|afk_timeout|game_cycle_completed|signal_received", at?: <unix_ts> }`
  - Validation: Chỉ chấp nhận room có status `ACTIVED`, validate reason hợp lệ
  - Behavior: Khi nhận hợp lệ → set `FULFILLED` với `end_reason`, `graceful_at`, `fulfilled_at`; lưu để cron phân biệt crash.
- `GET /rooms`
  - Response: `{ waiting: [...tickets OPENED...], matched: [...rooms states...] }`

### Ghi chú
- `room_id` do Agent sinh khi match; client không được cung cấp `room_id` khi submit ticket.
- Legacy endpoints (như `/create_room`) không còn khuyến nghị; vui lòng dùng API ticket ở trên.

## Nomad
- SDK: `github.com/hashicorp/nomad/api`
- Job `game-server-<room_id>`: driver `exec`, command `/usr/local/bin/server`, args: `["${NOMAD_PORT_http}", "<room_id>", "<bearer_token>"]`, dynamic port label `http`
- Double-check allocate:
  1) Kiểm tra Nomad allocation RUNNING/healthy.
  2) Readiness probe TCP connect đến room service sau `double_check_interval` (2s). Pass 2 lần probe mới set `ACTIVED`.
- Idempotency: ràng buộc 1 job/room; nếu retry trong allocate window, luôn kiểm tra/đọc lại job hiện có thay vì tạo job mới.
- Lock phân tán: `lock:room:allocate:<room_id>` với TTL ngắn (≈15s) để tránh race giữa agents.

## Cron & Consistency
- **Nguyên tắc tối thượng**: `count(RUNNING game-server jobs) == count(ACTIVED rooms)`
- **Dừng job terminal**: Room `DEAD`/`FULFILLED` → dừng job tương ứng ngay
- **Crash detection**: `ACTIVED` room không có running job → `DEAD(server_crash)`
- **Stray job cleanup**: Job chạy không có room `ACTIVED` tương ứng → dừng (chỉ game-server jobs)
- **Timeout handling**: `OPENED` room timeout → `DEAD(alloc_timeout)`
- **FULFILLED**: Chỉ từ `POST /rooms/:id/shutdown` hợp lệ (graceful), không tự động set

## UI
- `/ui`: HTML+JS, poll `/rooms` mỗi 3s; hiển thị Waiting (tickets), Matched/Actived (rooms); trạng thái room `OPENED|ACTIVED|DEAD|FULFILLED`

## TTL & Timeout (cấu hình)
- `allocate_ttl_seconds`: 120s mặc định. Hết hạn khi còn `OPENED` → set `DEAD` với `fail_reason=alloc_timeout`.
- `terminal_ttl_seconds`: 60s mặc định. TTL cho `DEAD` và `FULFILLED` để client thấy trạng thái cuối thay vì `ROOM_NOT_FOUND`.
- `double_check_interval_seconds`: 2s mặc định. Khoảng giữa hai lần check.
- `retry_backoff`: 1s, 2s, 4s (giới hạn trong allocate_ttl).
- `agent_bearer_token`: Token cho server-agent communication (mặc định "1234abcd").

## State machine & an toàn cạnh tranh
- Chuyển đổi hợp lệ: `OPENED → ACTIVED | DEAD`, `ACTIVED → FULFILLED | DEAD(server_crash)`. `DEAD` và `FULFILLED` là terminal, loại trừ nhau với `ACTIVED`.
- Dùng `state_rank` đơn điệu (OPENED=1 < ACTIVED=2 < DEAD=3 < FULFILLED=4) và CAS `version` khi update Redis để tránh race.
- Khi chuyển `DEAD`, đảm bảo cancel/cleanup Nomad job nếu đã tạo để tránh rò rỉ tài nguyên.

## Observability
- Events: phát sự kiện ở các mốc OPENED/ACTIVED/DEAD/FULFILLED cho audit; kèm `fail_reason`/`end_reason`.
- Metrics: `allocate_success_rate`, `allocate_time_ms`, `dead_rate` (trong đó `server_crash_rate`), `fulfilled_count`, breakdown theo `end_reason`.
