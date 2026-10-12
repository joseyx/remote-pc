//go:build !windows
// +build !windows

package system

import (
	"bufio"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func GetStats(includeTop ...bool) SystemStats {
	var totalGB, availGB, usedGB float64
	var totalPageGB, usedPageGB, availPageGB float64

	if runtime.GOOS == "linux" {
		totalGB, availGB, totalPageGB, availPageGB = readLinuxMemInfo()
	} else if runtime.GOOS == "darwin" {
		totalGB, availGB = readDarwinMemInfo()
	} else {
		totalGB = 16.0
		availGB = 8.0
	}

	usedGB = totalGB - availGB
	if usedGB < 0 {
		usedGB = 0
	}
	var pct int
	if totalGB > 0 {
		pct = int(math.Round((usedGB / totalGB) * 100))
	}

	usedPageGB = totalPageGB - availPageGB
	if usedPageGB < 0 {
		usedPageGB = 0
	}
	var pagePct int
	if totalPageGB > 0 {
		pagePct = int(math.Round((usedPageGB / totalPageGB) * 100))
	}

	stats := SystemStats{
		Memory: MemoryStats{
			TotalGB:     math.Round(totalGB*10) / 10,
			UsedGB:      math.Round(usedGB*10) / 10,
			FreeGB:      math.Round(availGB*10) / 10,
			Percent:     pct,
			TotalPageGB: math.Round(totalPageGB*10) / 10,
			UsedPageGB:  math.Round(usedPageGB*10) / 10,
			PagePercent: pagePct,
		},
		Services: []ServiceInfo{
			checkService("mysql", "MySQL 8.0 (Local)", 3306),
			checkService("aditum", "Aditum Main Dev", 8080),
			checkService("brif", "Brif Scrapper Web", 8000),
		},
		CheckedAt: time.Now(),
	}

	if len(includeTop) > 0 && includeTop[0] {
		stats.TopProcesses = getTopProcesses(10)
	}

	return stats
}

func readLinuxMemInfo() (totalGB, availGB, totalSwapGB, availSwapGB float64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 16.0, 8.0, 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var memTotalKB, memAvailKB, memFreeKB, swapTotalKB, swapFreeKB float64

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, _ := strconv.ParseFloat(parts[1], 64)

		switch key {
		case "MemTotal":
			memTotalKB = val
		case "MemAvailable":
			memAvailKB = val
		case "MemFree":
			memFreeKB = val
		case "SwapTotal":
			swapTotalKB = val
		case "SwapFree":
			swapFreeKB = val
		}
	}

	if memAvailKB == 0 {
		memAvailKB = memFreeKB
	}

	totalGB = memTotalKB / (1024 * 1024)
	availGB = memAvailKB / (1024 * 1024)
	totalSwapGB = swapTotalKB / (1024 * 1024)
	availSwapGB = swapFreeKB / (1024 * 1024)
	return
}

func readDarwinMemInfo() (totalGB, availGB float64) {
	// sysctl hw.memsize
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err == nil {
		bytes, parseErr := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if parseErr == nil {
			totalGB = bytes / (1024 * 1024 * 1024)
		}
	}
	if totalGB == 0 {
		totalGB = 16.0
	}
	// Estimate available as 40% if vm_stat parsing omitted
	availGB = totalGB * 0.4
	return
}

func getTopProcesses(limit int) []ProcessInfo {
	// Use standard POSIX ps command: ps -eo pid,rss,comm --sort=-rss
	cmd := exec.Command("ps", "-eo", "pid,rss,comm", "--sort=-rss")
	out, err := cmd.Output()
	if err != nil {
		// Fallback for macOS: ps -eo pid,rss,comm -m
		cmd = exec.Command("ps", "-eo", "pid,rss,comm")
		out, err = cmd.Output()
		if err != nil {
			return nil
		}
	}

	lines := strings.Split(string(out), "\n")
	var list []ProcessInfo

	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, _ := strconv.ParseUint(fields[0], 10, 32)
		rssKB, _ := strconv.ParseFloat(fields[1], 64)
		name := strings.Join(fields[2:], " ")
		ramMB := rssKB / 1024

		if ramMB > 15 {
			list = append(list, ProcessInfo{
				PID:   uint32(pid),
				Name:  name,
				RAMMB: math.Round(ramMB*10) / 10,
			})
		}
		if len(list) >= limit {
			break
		}
	}

	return list
}

func KillProcess(pid uint32) error {
	return syscall.Kill(int(pid), syscall.SIGKILL)
}
