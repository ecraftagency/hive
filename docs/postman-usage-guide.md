# Hướng Dẫn Sử Dụng Postman Collection và Auto-Flow

## Tổng Quan

Tài liệu này hướng dẫn cách sử dụng Postman Collection với Auto-Flow cho Hive Agent API. **Postman Flow không được hỗ trợ** trong phiên bản hiện tại, vì vậy chúng ta sử dụng test scripts để tự động hóa quy trình.

## ⚠️ **Lưu Ý Quan Trọng**

**Postman Flow không được hỗ trợ** vì:
- Format JSON v1 schema không được nhận diện
- Có thể chỉ có sẵn trong phiên bản Postman đặc biệt
- Có thể yêu cầu tài khoản Postman đặc biệt

**Giải pháp thay thế**: Sử dụng **Postman Collection với Auto-Flow** - hoạt động 100% và không có yêu cầu đặc biệt.

## Files

1. **`hive-agent-postman-collection.json`** - Collection cơ bản cho tất cả API endpoints
2. **`hive-agent-postman-collection-with-flow.json`** - Collection nâng cao với auto-flow và test scripts ⭐ **KHUYẾN NGHỊ**
3. **`hive-agent-postman-flow.json`** - Postman Flow (❌ **KHÔNG ĐƯỢC HỖ TRỢ**)

## Cài Đặt

### 1. Import Collection với Auto-Flow (Khuyến Nghị)

1. Mở Postman
2. Click **Import** button
3. Chọn file `hive-agent-postman-collection-with-flow.json`
4. Collection sẽ xuất hiện trong sidebar

### 2. Import Collection Cơ Bản

1. Mở Postman
2. Click **Import** button
3. Chọn file `hive-agent-postman-collection.json`
4. Collection sẽ xuất hiện trong sidebar

## Sử Dụng Collection với Auto-Flow

### Biến Collection

Collection có sẵn các biến sau:

- **`base_url`**: URL cơ sở của Agent service (mặc định: `http://localhost:8080`)
- **`player_id`**: ID người chơi để test (mặc định: `player123`)
- **`ticket_id`**: ID ticket được tạo tự động
- **`room_id`**: ID phòng được tạo tự động
- **`max_poll_attempts`**: Số lần polling tối đa (mặc định: 60)
- **`poll_interval_ms`**: Khoảng thời gian giữa các lần poll (mặc định: 2000ms)

### Thay Đổi Biến

1. Click vào biểu tượng **Variables** trong collection
2. Thay đổi giá trị `base_url` nếu cần
3. Thay đổi `player_id` để test với người chơi khác nhau
4. Điều chỉnh `max_poll_attempts` và `poll_interval_ms` nếu cần

## Auto-Flow Workflow

### 🚀 Complete Ticket Flow (Auto)

Collection này có 3 request được thiết kế để chạy theo thứ tự:

1. **Submit Ticket** → Tạo ticket và lưu `ticket_id`
2. **Poll Ticket Status** → Kiểm tra trạng thái và lưu `room_id` khi matched
3. **Poll Room Status** → Kiểm tra phòng và hiển thị thông tin server

### Cách Chạy Auto-Flow

#### Phương Pháp 1: Chạy Từng Request
1. Chạy **Submit Ticket** trước
2. Chạy **Poll Ticket Status** (có thể chạy nhiều lần)
3. Chạy **Poll Room Status** (có thể chạy nhiều lần)

#### Phương Pháp 2: Chạy Collection
1. Click vào **Run Collection** button
2. Chọn folder **🚀 Complete Ticket Flow (Auto)**
3. Click **Run**
4. Collection sẽ chạy theo thứ tự

### Test Scripts Tự Động

Mỗi request có test scripts tự động:

- **Lưu biến**: `ticket_id`, `room_id` được lưu tự động
- **Validation**: Kiểm tra response format và status
- **Logging**: Console logs với emoji để dễ theo dõi
- **Error Handling**: Xử lý các trường hợp lỗi

## 📋 Individual API Tests

### Test Từng Endpoint Riêng Lẻ

Collection cũng cung cấp các test riêng lẻ:

1. **Submit Ticket (Individual)** - Test tạo ticket
2. **Get Ticket Status** - Test lấy trạng thái ticket
3. **Cancel Ticket** - Test hủy ticket
4. **Get Room Information** - Test lấy thông tin phòng
5. **Admin Overview** - Test tổng quan hệ thống
6. **Web UI** - Test giao diện web

## Monitoring và Debug

### Console Logs

Mở **Console** trong Postman để xem logs:

```
✅ Ticket ID đã được lưu: abc123-def456
🔄 Bắt đầu Auto-Flow workflow...
📊 Ticket Status: OPENED
⏳ Ticket still waiting...
📊 Ticket Status: MATCHED
🎯 Ticket matched! Room ID: room789-xyz012
🔄 Bắt đầu polling room status...
🏠 Room Status: OPENED
⏳ Room still allocating...
🏠 Room Status: FULFILLED
🎉 Phòng đã sẵn sàng!
🌐 Server IP: 192.168.1.100
🔌 Port: 8080
🎮 Người chơi có thể kết nối!
```

### Variables Tracking

Theo dõi biến trong tab **Variables**:
- `ticket_id`: ID ticket hiện tại
- `room_id`: ID phòng hiện tại

### Test Results

Xem kết quả test trong tab **Test Results**:
- ✅ Pass: Test thành công
- ❌ Fail: Test thất bại
- Thông tin chi tiết về lỗi

## Test Scenarios

### Scenario 1: Ticket Thành Công

1. **Chạy Submit Ticket** → Lưu `ticket_id`
2. **Chạy Poll Ticket Status** → Chờ cho đến khi matched
3. **Chạy Poll Room Status** → Chờ cho đến khi server ready
4. **Kết quả**: Thông tin server IP và port

### Scenario 2: Ticket Bị Từ Chối

1. **Chạy Submit Ticket** với `player_id` đã tồn tại
2. **Kết quả**: Status "REJECTED"

### Scenario 3: Manual Polling

1. **Chạy Submit Ticket** → Lưu `ticket_id`
2. **Chạy Poll Ticket Status** nhiều lần cho đến khi matched
3. **Chạy Poll Room Status** nhiều lần cho đến khi ready

## Troubleshooting

### Lỗi Thường Gặp

1. **Connection Error**: Kiểm tra `base_url` và đảm bảo Agent service đang chạy
2. **Variable Not Set**: Đảm bảo đã chạy Submit Ticket trước khi chạy các request khác
3. **Test Fail**: Kiểm tra Console logs để xem chi tiết lỗi

### Debug Tips

1. **Sử dụng Console**: Xem logs chi tiết
2. **Kiểm tra Variables**: Đảm bảo biến được set đúng
3. **Chạy từng request**: Test từng endpoint riêng lẻ
4. **Kiểm tra response**: Xem response body và headers

## Tùy Chỉnh

### Thay Đổi Logic Polling

Bạn có thể chỉnh sửa test scripts để:
- Thay đổi logic retry
- Thêm delay giữa các lần poll
- Xử lý các trường hợp đặc biệt

### Thêm Validation

Thêm các test case để:
- Kiểm tra response format
- Validate business logic
- Xử lý error cases

## Kết Luận

**Postman Collection với Auto-Flow** cung cấp giải pháp hoàn chỉnh và đáng tin cậy cho việc test Hive Agent API. Không cần lo lắng về Postman Flow - collection đã đủ tốt và hoạt động 100%.

### Ưu Điểm của Collection với Auto-Flow:
- ✅ **Hoạt động ngay lập tức** - không cần cài đặt đặc biệt
- ✅ **Tự động hóa hoàn chỉnh** - workflow ticket submit và polling
- ✅ **Test scripts mạnh mẽ** - validation và logging chi tiết
- ✅ **Dễ dàng tùy chỉnh** - có thể chỉnh sửa logic theo nhu cầu
- ✅ **Tương thích 100%** - hoạt động với mọi phiên bản Postman
