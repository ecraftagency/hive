package svrmgr

import (
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/nomad/api"
)

// Manager chịu trách nhiệm tương tác với Nomad API
type Manager struct {
	client *api.Client
}

// Tạm thời map private IP -> public IP cho phản hồi RoomInfo
var privateToPublicIP = map[string]string{
	"172.26.15.163": "52.221.213.97",
}

// RoomInfo mô tả thông tin phân bổ của một room/job
type RoomInfo struct {
	RoomID       string         `json:"room_id"`
	AllocationID string         `json:"allocation_id"`
	NodeID       string         `json:"node_id"`
	HostIP       string         `json:"host_ip"`
	Ports        map[string]int `json:"ports"`
}

// New khởi tạo Nomad client với địa chỉ server
func New(address string) (*Manager, error) {
	cli, err := api.NewClient(&api.Config{Address: address})
	if err != nil {
		return nil, err
	}
	return &Manager{client: cli}, nil
}

// RunGameServer tạo và đăng ký một batch job cho game server với dynamic port
func (m *Manager) RunGameServer(roomID string) error {
	jobName := fmt.Sprintf("game-server-%s", roomID)
	jobType := "batch"
	count := 1
	tgName := "game-server"
	taskName := "server"
	driver := "exec"

	// Task group & task
	tg := api.NewTaskGroup(tgName, count)
	task := api.NewTask(taskName, driver)
	task.SetConfig("command", "/usr/local/bin/server")
	// args: 1) dynamic port 2) roomID
	task.SetConfig("args", []string{"${NOMAD_PORT_http}", roomID})

	// Log rotation config
	maxFiles := 5
	maxFileSizeMB := 10
	logsDisabled := false
	task.LogConfig = &api.LogConfig{
		MaxFiles:      &maxFiles,
		MaxFileSizeMB: &maxFileSizeMB,
		Disabled:      &logsDisabled,
	}

	// Dynamic host port with label "http"
	task.Require(&api.Resources{
		Networks: []*api.NetworkResource{
			{
				DynamicPorts: []api.Port{
					{Label: "http"},
				},
			},
		},
	})

	tg.Tasks = []*api.Task{task}

	job := &api.Job{
		ID:          &roomID,
		Name:        &jobName,
		Type:        &jobType,
		Datacenters: []string{"dc1"},
		TaskGroups:  []*api.TaskGroup{tg},
	}

	_, _, err := m.client.Jobs().Register(job, nil)
	return err
}

// GetRoomInfo trả về IP host và cổng được Nomad cấp phát dựa trên JobID (roomID)
func (m *Manager) GetRoomInfo(roomID string) (*RoomInfo, error) {
	stubs, _, err := m.client.Jobs().Allocations(roomID, false, nil)
	if err != nil {
		return nil, err
	}
	if len(stubs) == 0 {
		return nil, fmt.Errorf("no allocations for job %s", roomID)
	}
	// Chọn alloc đang chạy nếu có, không thì lấy alloc đầu tiên
	var chosen *api.AllocationListStub
	for _, s := range stubs {
		if s.ClientStatus == "running" {
			chosen = s
			break
		}
	}
	if chosen == nil {
		chosen = stubs[0]
	}

	alloc, _, err := m.client.Allocations().Info(chosen.ID, nil)
	if err != nil {
		return nil, err
	}

	ports := map[string]int{}
	if alloc != nil && alloc.AllocatedResources != nil {
		ar := alloc.AllocatedResources
		// 1) Shared.Ports (PortMapping)
		for _, pm := range ar.Shared.Ports {
			if pm.Label != "" {
				if pm.Value != 0 {
					ports[pm.Label] = pm.Value
				} else if pm.To != 0 {
					ports[pm.Label] = pm.To
				}
			}
		}
		// 2) Shared.Networks dynamic ports
		for _, netr := range ar.Shared.Networks {
			for _, p := range netr.DynamicPorts {
				v := p.Value
				if v == 0 {
					v = p.To
				}
				if p.Label != "" && v != 0 {
					ports[p.Label] = v
				}
			}
		}
		// 3) Per-task networks dynamic ports
		for _, tr := range ar.Tasks {
			for _, netr := range tr.Networks {
				for _, p := range netr.DynamicPorts {
					v := p.Value
					if v == 0 {
						v = p.To
					}
					if p.Label != "" && v != 0 {
						ports[p.Label] = v
					}
				}
			}
		}
	}

	nodeIP := ""
	if alloc != nil && alloc.NodeID != "" {
		n, _, nerr := m.client.Nodes().Info(alloc.NodeID, nil)
		if nerr == nil && n != nil {
			// Ưu tiên attribute phổ biến
			if ip, ok := n.Attributes["unique.network.ip-address"]; ok && ip != "" {
				nodeIP = ip
			} else if addr := n.HTTPAddr; addr != "" {
				host, _, _ := net.SplitHostPort(addr)
				if host == "" {
					// Fallback nếu không có port
					parts := strings.Split(addr, ":")
					if len(parts) > 0 {
						host = parts[0]
					}
				}
				nodeIP = host
			}
		}
	}

	// Áp dụng map private->public nếu có
	if pub, ok := privateToPublicIP[nodeIP]; ok && pub != "" {
		nodeIP = pub
	}

	info := &RoomInfo{
		RoomID:       roomID,
		AllocationID: chosen.ID,
		NodeID:       alloc.NodeID,
		HostIP:       nodeIP,
		Ports:        ports,
	}
	return info, nil
}
