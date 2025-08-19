# Hive - Dedicated Server Management System

Hệ thống quản lý dedicated server đơn giản với 3 component chính: Agent, Game Server và Client (CLI/Web).

## Kiến trúc & Tech stack
- Go 1.21+, Gin
- Nomad (HashiCorp) orchestrate job `game-server-<room_id>` (driver `exec`, dynamic port)
- Redis lưu state: pending queue, room state, chống trùng vé
- UI: HTML+JS render từ Go (không framework)

## Thư mục
- `cmd/agent`: Agent HTTP + UI `/ui`
- `cmd/server`: Game Server HTTP + UI `/`
- `cmd/client`: CLI client
- `cmd/web`: Web client (proxy tới Agent)
- `pkg/svrmgr`: Nomad SDK wrapper
- `pkg/store`: Redis store
- `pkg/mm`: Matchmaking logic
- `pkg/cron`: Đồng bộ Redis ↔ Nomad
- `pkg/ui`: Template HTML/JS
- `docs`: Tài liệu kiến trúc và từng component

## Build
```
make build        # build tất cả
make agent        # build agent
make server       # build server
make client       # build CLI client
make web          # build web client
```

## Deploy (remote host ubuntu@52.221.213.97)
```
make deploy-agent
make deploy-server
make deploy-web
```

## Chạy cục bộ
```
./bin/agent      # listen :8080
./bin/server 8081 flask   # ví dụ server port 8081 room_id=flask
./bin/client     # CLI
./bin/web        # Web client listen :8082
```

## Endpoints chính
- Agent
  - `GET /create_room?room_id&player_id`
  - `GET /join_room?player_id`
  - `GET /room/:room_id`
  - `GET /rooms`
  - `GET /ui` (dashboard)
- Server
  - `GET /heartbeat?player_id`
  - `GET /players`
  - `GET /` (UI)
- Web client (local):
  - Giao diện nhập `player_id`, `room_name`, tạo/phối ghép phòng, poll room info, heartbeat trực tiếp server

## Tài liệu
- `docs/arch.md`: Kiến trúc tổng thể & tech stack
- `docs/agent.md`: Chức năng Agent, API, Redis, đồng bộ
- `docs/server.md`: Game server, CORS, shutdown
- `docs/client.md`: Client CLI và Web
