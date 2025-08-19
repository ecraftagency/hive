# Client

## CLI
- (Hiện hành) CLI mẫu vẫn có `create_room <room_name>`; khi áp dụng matchmaking mới, CLI sẽ chuyển sang chỉ `join` (submit ticket bằng `player_id`) và nhận `room_id` do Agent sinh.

## Web Client (cập nhật)
- Form nhập `player_id` (Random Name)
- Nút & Flow:
  - Join → submit ticket: `POST /tickets { player_id }`
  - Poll ticket: `GET /tickets/:ticket_id` → khi `MATCHED` nhận `room_id`
  - Poll room: `GET /rooms/:room_id` → đến khi `FULFILLED` (nhận server) hoặc `DEAD` (fail_reason), dừng
  - Cancel: `POST /tickets/:ticket_id/cancel` khi ticket còn `OPENED`
- Heartbeat: gọi trực tiếp `http://<server_ip>:<port>/heartbeat?player_id=...` (CORS bật trên server) mỗi 3s; log ok/failed
- UI: Dark theme, hiển thị URL server dạng link; có thể thêm backoff polling (khuyến nghị)
