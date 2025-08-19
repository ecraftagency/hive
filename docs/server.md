# Game Server

## Khởi chạy
- Nhận tham số dòng lệnh:
  - arg1: `port` (Nomad dynamic host port)
  - arg2: `room_id`
- Ví dụ: `/usr/local/bin/server 31695 flask`

## CORS
- Bật CORS cho mọi `Origin`, cho phép `GET, POST, OPTIONS` và headers cơ bản; trả `204` cho preflight OPTIONS.

## Endpoints
- `GET /heartbeat?player_id=...`: ghi nhận heartbeat, cập nhật `last_seen` người chơi
- `GET /players`: trả `{ players: [{player_id, state, last_seen_unix}], room_id }`
- `GET /`: trang UI hiển thị room, số lượng connected/disconnected, bảng players, log

## Shutdown
- Bắt đầu kiểm tra sau `10s` (initial grace)
- Nếu chưa ai heartbeat trong 10s đầu → shutdown
- Nếu có người chơi nhưng bất kỳ player nào không heartbeat > `10s` → shutdown

## UI
- Giao diện nền tối, bảng players, mục log; tự động refresh `/players` mỗi `2s`
