//go:build windows
// +build windows

package system

import (
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

type ProcessMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

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

func GetStats() SystemStats {
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	_, _, _ = procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))

	totalGB := float64(mem.TotalPhys) / (1024 * 1024 * 1024)
	availGB := float64(mem.AvailPhys) / (1024 * 1024 * 1024)
	usedGB := totalGB - availGB
	var pct int
	if totalGB > 0 {
		pct = int(math.Round((usedGB / totalGB) * 100))
	}

	totalPageGB := float64(mem.TotalPageFile) / (1024 * 1024 * 1024)
	availPageGB := float64(mem.AvailPageFile) / (1024 * 1024 * 1024)
	usedPageGB := totalPageGB - availPageGB
	var pagePct int
	if totalPageGB > 0 {
		pagePct = int(math.Round((usedPageGB / totalPageGB) * 100))
	}

	services := []ServiceInfo{
		checkService("mysql", "MySQL Local", 3306),
		checkService("redis", "Redis (Celery)", 6379),
		checkService("flower", "Flower Dashboard", 5555),
	}

	return SystemStats{
		Memory: MemoryStats{
			TotalGB:     math.Round(totalGB*10) / 10,
			UsedGB:      math.Round(usedGB*10) / 10,
			FreeGB:      math.Round(availGB*10) / 10,
			Percent:     pct,
			TotalPageGB: math.Round(totalPageGB*10) / 10,
			UsedPageGB:  math.Round(usedPageGB*10) / 10,
			PagePercent: pagePct,
		},
		Services:     services,
		TopProcesses: GetTopProcesses(5),
		CheckedAt:    time.Now(),
	}
}

func GetTopProcesses(limit int) []ProcessInfo {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	var list []ProcessInfo
	if err := windows.Process32First(snapshot, &entry); err == nil {
		for {
			pid := entry.ProcessID
			if pid > 4 {
				name := windows.UTF16ToString(entry.ExeFile[:])
				hProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
				if err == nil {
					var pmc ProcessMemoryCounters
					pmc.CB = uint32(unsafe.Sizeof(pmc))
					r1, _, _ := procGetProcessMemoryInfo.Call(uintptr(hProcess), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.CB))
					windows.CloseHandle(hProcess)
					if r1 != 0 {
						mb := float64(pmc.WorkingSetSize) / (1024 * 1024)
						if mb > 15 {
							list = append(list, ProcessInfo{
								PID:   pid,
								Name:  name,
								RAMMB: math.Round(mb*10) / 10,
							})
						}
					}
				}
			}
			if err := windows.Process32Next(snapshot, &entry); err != nil {
				break
			}
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].RAMMB > list[j].RAMMB
	})

	if len(list) > limit {
		return list[:limit]
	}
	return list
}

func KillProcess(pid uint32) error {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return p.Kill()
}

func checkService(id, name string, port int) ServiceInfo {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return ServiceInfo{ID: id, Name: name, Port: port, Running: false}
	}
	_ = conn.Close()
	return ServiceInfo{ID: id, Name: name, Port: port, Running: true, PingMs: time.Since(start).Milliseconds()}
}
