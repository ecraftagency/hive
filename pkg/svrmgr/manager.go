package svrmgr

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
)

// Manager chịu trách nhiệm tương tác với Nomad API
type Manager struct {
	client      *api.Client
	datacenters []string
	bearerToken string
}

// SetDatacenters sets the datacenters for Nomad jobs
func (m *Manager) SetDatacenters(datacenters []string) {
	m.datacenters = datacenters
}

// SetBearerToken sets bearer token to inject to server args
func (m *Manager) SetBearerToken(token string) { m.bearerToken = token }

// IP mapping config - should be loaded from config
type IPMapping struct {
	PrivateIP string `json:"private_ip"`
	PublicIP  string `json:"public_ip"`
}

// IPMappingConfig holds IP mapping configuration
type IPMappingConfig struct {
	Mappings []IPMapping `json:"mappings"`
}

// Default empty mapping - should be populated from config
var ipMappingConfig = &IPMappingConfig{
	Mappings: []IPMapping{},
}

// SetIPMappingConfig sets the IP mapping configuration. If none provided,
// use a sensible default mapping for production.
func SetIPMappingConfig(config *IPMappingConfig) {
	if config != nil && len(config.Mappings) > 0 {
		ipMappingConfig = config
		return
	}
	// Fallback default when no ENV/IP mappings provided
	ipMappingConfig = &IPMappingConfig{
		Mappings: []IPMapping{{PrivateIP: "172.26.15.163", PublicIP: "52.221.213.97"}},
	}
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

// CountRunningJobsByNamePrefix đếm số job có tên bắt đầu bằng prefix và đang có allocation chạy
func (m *Manager) CountRunningJobsByNamePrefix(prefix string) (int, error) {
	if prefix == "" {
		prefix = "game-server-"
	}
	jobs, _, err := m.client.Jobs().List(nil)
	if err != nil {
		return 0, err
	}
	targetJobIDs := map[string]struct{}{}
	for _, j := range jobs {
		if j == nil {
			continue
		}
		if j.Name != "" && strings.HasPrefix(j.Name, prefix) {
			if j.ID != "" {
				targetJobIDs[j.ID] = struct{}{}
			}
		}
	}
	count := 0
	for jobID := range targetJobIDs {
		stubs, _, err := m.client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			continue
		}
		for _, s := range stubs {
			if s != nil && s.ClientStatus == "running" {
				count++
				break
			}
		}
	}
	return count, nil
}

// RunGameServer tạo và đăng ký một batch job cho game server với dynamic port
func (m *Manager) RunGameServer(roomID string) error {
	jobName := fmt.Sprintf("game-server-%s", roomID)
	jobType := "batch"
	count := 1
	gName := "game-server"
	taskName := "server"
	driver := "raw_exec"

	// Task group & task
	tg := api.NewTaskGroup(gName, count)
	task := api.NewTask(taskName, driver)
	task.SetConfig("command", "/usr/local/bin/boardserver/server.x86_64")
	// args: 1) dynamic port 2) roomID 3) bearer token
	bearer := m.bearerToken
	if bearer == "" {
		bearer = "1234abcd"
	}
	task.SetConfig("args", []string{"-port", "${NOMAD_PORT_http}", "-serverId", roomID, "-token", bearer, "-nographics", "-batchmode"})

	// Log rotation config
	maxFiles := 5
	maxFileSizeMB := 10
	logsDisabled := false
	task.LogConfig = &api.LogConfig{
		MaxFiles:      &maxFiles,
		MaxFileSizeMB: &maxFileSizeMB,
		Disabled:      &logsDisabled,
	}

	// Restart policy - no restart attempts
	attempts := 0
	restartMode := "fail"
	task.RestartPolicy = &api.RestartPolicy{
		Attempts: &attempts,
		Mode:     &restartMode,
	}

	// Dynamic host port with label "http"
	cpu := 100
	memoryMB := 100
	diskMB := 10
	task.Require(&api.Resources{
		CPU:      &cpu,
		MemoryMB: &memoryMB,
		DiskMB:   &diskMB,
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
		Datacenters: m.datacenters, // Set from config
		TaskGroups:  []*api.TaskGroup{tg},
	}

	_, _, err := m.client.Jobs().Register(job, nil)
	return err
}

// RunGameServerV2 tạo và đăng ký một batch job cho game server với tùy chỉnh resources, command và arguments
func (m *Manager) RunGameServerV2(roomID string, cpu int, memoryMB int, command string, args []string) error {
	jobName := fmt.Sprintf("game-server-%s", roomID)
	jobType := "batch"
	count := 1
	tgName := "game-server"
	taskName := "server"
	driver := "raw_exec"

	// Task group & task
	tg := api.NewTaskGroup(tgName, count)
	task := api.NewTask(taskName, driver)
	task.SetConfig("command", command)

	// Use custom args if provided, otherwise default to port and roomID
	if len(args) > 0 {
		task.SetConfig("args", args)
	} else {
		bearer := m.bearerToken
		if bearer == "" {
			bearer = "1234abcd"
		}
		task.SetConfig("args", []string{"-port", "${NOMAD_PORT_http}", "-session=", roomID, "-token", bearer, "-nographics", "-batchmode"})
	}

	// Log rotation config
	maxFiles := 5
	maxFileSizeMB := 10
	logsDisabled := false
	task.LogConfig = &api.LogConfig{
		MaxFiles:      &maxFiles,
		MaxFileSizeMB: &maxFileSizeMB,
		Disabled:      &logsDisabled,
	}

	// Restart policy - no restart attempts
	attempts := 0
	restartMode := "fail"
	task.RestartPolicy = &api.RestartPolicy{
		Attempts: &attempts,
		Mode:     &restartMode,
	}

	// Resources with custom CPU, Memory and Disk
	diskMB := 10
	resources := &api.Resources{
		CPU:      &cpu,
		MemoryMB: &memoryMB,
		DiskMB:   &diskMB,
		Networks: []*api.NetworkResource{
			{
				DynamicPorts: []api.Port{
					{Label: "http"},
				},
			},
		},
	}
	task.Require(resources)

	tg.Tasks = []*api.Task{task}

	job := &api.Job{
		ID:          &roomID,
		Name:        &jobName,
		Type:        &jobType,
		Datacenters: m.datacenters,
		TaskGroups:  []*api.TaskGroup{tg},
		Meta:        map[string]string{"created_at": time.Now().UTC().Format(time.RFC3339)},
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
	// Chỉ lấy alloc đang chạy, không lấy alloc đã stop/failed
	var chosen *api.AllocationListStub
	for _, s := range stubs {
		if s.ClientStatus == "running" {
			chosen = s
			break
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no running allocation for job %s", roomID)
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
	for _, mapping := range ipMappingConfig.Mappings {
		if mapping.PrivateIP == nodeIP && mapping.PublicIP != "" {
			nodeIP = mapping.PublicIP
			break
		}
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
