package system

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type ProcessInfo struct {
	PID   uint32  `json:"pid"`
	Name  string  `json:"name"`
	RAMMB float64 `json:"ramMB"`
}

type MemoryStats struct {
	TotalGB     float64 `json:"totalGB"`
	UsedGB      float64 `json:"usedGB"`
	FreeGB      float64 `json:"freeGB"`
	Percent     int     `json:"percent"`
	TotalPageGB float64 `json:"totalPageGB"`
	UsedPageGB  float64 `json:"usedPageGB"`
	PagePercent int     `json:"pagePercent"`
}

type ServiceTarget struct {
	ID   string
	Name string
	Port int
}

type ServiceInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Running bool   `json:"running"`
	PingMs  int64  `json:"pingMs"`
}

type SystemStats struct {
	Memory       MemoryStats   `json:"memory"`
	Services     []ServiceInfo `json:"services"`
	TopProcesses []ProcessInfo `json:"topProcesses"`
	CheckedAt    time.Time     `json:"checkedAt"`
}

func checkService(id, name string, port int) ServiceInfo {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 40*time.Millisecond)
	if err != nil {
		return ServiceInfo{ID: id, Name: name, Port: port, Running: false}
	}
	_ = conn.Close()
	return ServiceInfo{ID: id, Name: name, Port: port, Running: true, PingMs: time.Since(start).Milliseconds()}
}

func CheckServices(targets []ServiceTarget) []ServiceInfo {
	results := make([]ServiceInfo, len(targets))
	var wg sync.WaitGroup
	wg.Add(len(targets))
	for i, t := range targets {
		go func(idx int, target ServiceTarget) {
			defer wg.Done()
			results[idx] = checkService(target.ID, target.Name, target.Port)
		}(i, t)
	}
	wg.Wait()
	return results
}
