# Agent

## Nhiệm vụ (cập nhật)
- Nhận ticket tham gia (join) từ client chỉ với `player_id` (không nhập `room_id`).
- Ghép phòng (match) → khi match found, Agent tự sinh `room_id` (UUID) và tạo room OPENED.
- Khởi chạy Nomad job `game-server-<room_id>` với args `[${NOMAD_PORT_http}, room_id]`, cập nhật state room (server ip/port, allocation_id) vào Redis khi alloc xong.
- Cung cấp API truy vấn ticket/room; cron đồng bộ Redis ↔ Nomad theo nguyên tắc fulfilled rooms == running jobs; server dừng → chỉ xóa room khỏi Redis (không dừng job để giữ log).
- UI `/ui`: hiển thị Waiting/Matched, auto-refresh.

## Lưu trữ (tóm tắt)
- Tickets: `mm:ticket:<id>` (TTL 120s), `mm:tickets:opened`, `mm:players:pending`
- Rooms: `mm:room:opened:<room_id>`, `mm:room:fulfilled:<room_id>`, `mm:room:dead:<room_id>`
- Index: `mm:rooms:opened`, `mm:rooms:fulfilled`, `mm:rooms:dead`

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
  - Response: `{ status: "OPENED"|"FULFILLED"|"DEAD", server?, fail_reason?, players }`
- `GET /rooms`
  - Response: `{ waiting: [...tickets OPENED...], matched: [...rooms states...] }`

### Ghi chú
- `room_id` do Agent sinh khi match; client không được cung cấp `room_id` khi submit ticket.
- Legacy endpoints (như `/create_room`) không còn khuyến nghị; vui lòng dùng API ticket ở trên.

## Nomad
- SDK: `github.com/hashicorp/nomad/api`
- Job `game-server-<room_id>`: driver `exec`, command `/usr/local/bin/server`, args: `["${NOMAD_PORT_http}", "<room_id>"]`, dynamic port label `http`

## UI
- `/ui`: HTML+JS, poll `/rooms` mỗi 3s; hiển thị Waiting (tickets) và Matched (rooms), trạng thái room `OPENED|FULFILLED|DEAD`
