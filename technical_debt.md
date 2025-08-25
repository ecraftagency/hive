## Technical Debt – Agent & Reconnect (MVP)

### Flaws / Limitations
- Lookup đang iterate toàn bộ active rooms từ Redis → O(N rooms). Chưa có index `player_id -> room_id`.
- Không có bảo mật cho reconnect (chủ đích), có thể mạo danh `player_id` nếu biết trước.
- Endpoint lookup trả 409 khi phát hiện `player_id` xuất hiện ở nhiều ACTIVED rooms (invariant violated), chưa có auto-heal.
- Enforce uniqueness khi ACTIVED chỉ kiểm ở thời điểm set trạng thái; có race hiếm gặp giữa nhiều allocations đồng thời.
- Không có TTL hay lịch sử reconnect; chỉ cho phép reconnect khi room còn ACTIVED.

### Potential Race Conditions
- Room A set ACTIVED gần đồng thời với Room B chứa cùng player: có cửa sổ trước khi check uniqueness, có thể cần lock theo `player_id`.
- Agent lookup ngay sau ACTIVED nhưng trước khi server thực sự lắng nghe /heartbeat → client reconnect có thể fail tạm thời.
- Server shutdown do size==0 đúng lúc client bắt đầu reconnect → cần grace dài hơn ở phía SDK nếu cần.

### Observability Gaps
- Chưa có metric đếm lookup/reconnect thành công/thất bại.
- Logs chưa chuẩn hóa (structured) quanh lookup và uniqueness violations.

### Optimizations (Later)
- Thêm index Redis: `mm:player2room:<player_id> = room_id` khi ACTIVED; cập nhật/xóa khi FULFILLED/DEAD.
- Lock phân tán theo `player_id` khi chuyển room → ACTIVED để loại race.
- Thêm reconnect window với TTL ngắn sau FULFILLED (410) nếu cần trải nghiệm tốt hơn.
- Thêm cache no-store headers chuẩn hơn và rate-limit lookup.

### Server Behavior
- Hiện shutdown khi `size==0` sau `InitialGrace`. Chưa có confirm nhiều chu kỳ liên tiếp (debounce) ngoài PollInterval.
- Endgame giả lập kéo dài 3 phút: có thể cấu hình qua ENV sau.


