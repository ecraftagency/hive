package main

import (
	"fmt"
	"log"
	"time"

	"hive/pkg/svrmgr"
)

func main() {
	// Kết nối đến Nomad server
	nomadAddr := "http://52.221.213.97:4646"
	manager, err := svrmgr.New(nomadAddr)
	if err != nil {
		log.Fatalf("Không thể kết nối đến Nomad: %v", err)
	}

	// Set datacenters
	manager.SetDatacenters([]string{"dc1"})

	// Set IP mapping config
	ipConfig := &svrmgr.IPMappingConfig{
		Mappings: []svrmgr.IPMapping{{PrivateIP: "172.26.15.163", PublicIP: "52.221.213.97"}},
	}
	svrmgr.SetIPMappingConfig(ipConfig)

	fmt.Println("🧪 Test Server Manager với Nomad")

	// Test RunGameServer
	fmt.Println("Testing RunGameServer...")
	roomID1 := fmt.Sprintf("test-room-%d", time.Now().Unix())
	err = manager.RunGameServerV2(roomID1, 100, 100, "/usr/local/bin/boardserver/server.x86_64", []string{"-port", "${NOMAD_PORT_http}", "-serverId", roomID1, "-token", "1234abcd", "-nographics", "-batchmode"})
	if err != nil {
		fmt.Printf("RunGameServer failed: %v\n", err)
	} else {
		fmt.Println("RunGameServer succeeded")
	}

	// Đợi job được allocate
	time.Sleep(3 * time.Second)

	// Kiểm tra thông tin room
	roomInfo1, err := manager.GetRoomInfo(roomID1)
	if err != nil {
		log.Printf("❌ Lỗi lấy thông tin room: %v", err)
	} else {
		fmt.Printf("📊 Room Info: %+v\n", roomInfo1)
	}

	// Test 2: Tạo game server với RunGameServerV2 (custom resources)
	fmt.Println("\n2️⃣ Test RunGameServerV2...")
	roomID2 := fmt.Sprintf("test-room-v2-%d", time.Now().Unix())
	err = manager.RunGameServerV2(roomID2, 200, 200, "/usr/local/bin/boardserver/server.x86_64", []string{"-port", "${NOMAD_PORT_http}", "-serverId", roomID2, "-token", "1234abcd", "-nographics", "-batchmode"})
	if err != nil {
		log.Printf("❌ Lỗi tạo game server V2: %v", err)
	} else {
		fmt.Printf("✅ Đã tạo job V2: %s\n", roomID2)
	}

	// Đợi job được allocate
	time.Sleep(3 * time.Second)

	// Kiểm tra thông tin room V2
	roomInfo2, err := manager.GetRoomInfo(roomID2)
	if err != nil {
		log.Printf("❌ Lỗi lấy thông tin room V2: %v", err)
	} else {
		fmt.Printf("📊 Room V2 Info: %+v\n", roomInfo2)
	}

	// Disk constraint
	fmt.Println("\n💾 Disk constraint đã được set: 10MB cho tất cả jobs")

	// Cleanup (optional)
	// Gợi ý: thêm method Deregister vào svrmgr nếu muốn cleanup từ đây

	fmt.Println("\n✅ Test hoàn thành!")
}
