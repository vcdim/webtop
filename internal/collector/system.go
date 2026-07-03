package collector

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type ProcEntry struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	Command string  `json:"command"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
}

type MemInfo struct {
	TotalMB     int     `json:"total_mb"`
	UsedMB      int     `json:"used_mb"`
	AvailableMB int     `json:"available_mb"`
	UsedPct     float64 `json:"used_pct"`
}

type SysData struct {
	NumCPU   int         `json:"num_cpu"`
	CPUTotal float64     `json:"cpu_total"`
	User     float64     `json:"user"`
	System   float64     `json:"system"`
	IOWait   float64     `json:"iowait"`
	Idle     float64     `json:"idle"`
	Cores    []float64   `json:"cores"`
	LoadAvg  [3]float64  `json:"load_avg"`
	Mem      MemInfo     `json:"mem"`
	Procs    []ProcEntry `json:"procs"`
}

// cpuTimes holds the raw jiffie counters from one /proc/stat line.
type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal float64
}

func (c cpuTimes) total() float64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func (c cpuTimes) busy() float64 {
	return c.total() - c.idle - c.iowait
}

// Previous /proc/stat snapshot, needed to diff cumulative counters into a rate.
// Guarded because broadcastLoop and sendAll can call CollectSystem concurrently.
var (
	prevMu   sync.Mutex
	prevCPU  cpuTimes            // aggregate
	prevCore map[int]cpuTimes    // per-core
	havePrev bool
)

// CollectSystem gathers CPU, memory, load, and top processes.
func CollectSystem() (*SysData, error) {
	if runtime.GOOS == "darwin" {
		return collectSystemDarwin()
	}
	return collectSystemLinux()
}

// --- Linux ---

func collectSystemLinux() (*SysData, error) {
	agg, cores, err := readProcStat()
	if err != nil {
		return nil, err
	}

	sd := &SysData{}
	computeCPUUsage(sd, agg, cores)
	sd.LoadAvg = readLoadAvg()
	sd.Mem = readMemInfo()
	sd.Procs = topProcs("ps", "-eo", "pid,user:32,pcpu,pmem,comm", "--sort=-pcpu")

	return sd, nil
}

// computeCPUUsage fills CPUTotal, the user/system/iowait/idle split, per-core
// utilization, and NumCPU by diffing the given snapshot against the previous one.
// Used by both the Linux (/proc/stat) and macOS (host_processor_info) paths.
func computeCPUUsage(sd *SysData, agg cpuTimes, cores []cpuTimes) {
	prevMu.Lock()
	defer prevMu.Unlock()

	sd.NumCPU = len(cores)
	sd.Cores = make([]float64, len(cores))
	if havePrev {
		sd.CPUTotal, sd.User, sd.System, sd.IOWait, sd.Idle = cpuPct(prevCPU, agg)
		for i := 0; i < len(cores); i++ {
			if p, ok := prevCore[i]; ok {
				busy, _, _, _, _ := cpuPct(p, cores[i])
				sd.Cores[i] = busy
			}
		}
	}
	prevCPU = agg
	prevCore = make(map[int]cpuTimes, len(cores))
	for i, c := range cores {
		prevCore[i] = c
	}
	havePrev = true
}

// cpuPct returns busy%, user%, system%, iowait%, idle% between two snapshots.
func cpuPct(prev, cur cpuTimes) (busy, user, system, iowait, idle float64) {
	dt := cur.total() - prev.total()
	if dt <= 0 {
		return 0, 0, 0, 0, 0
	}
	busy = (cur.busy() - prev.busy()) / dt * 100
	user = (cur.user + cur.nice - prev.user - prev.nice) / dt * 100
	system = (cur.system + cur.irq + cur.softirq - prev.system - prev.irq - prev.softirq) / dt * 100
	iowait = (cur.iowait - prev.iowait) / dt * 100
	idle = (cur.idle - prev.idle) / dt * 100
	return clampPct(busy), clampPct(user), clampPct(system), clampPct(iowait), clampPct(idle)
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func readProcStat() (agg cpuTimes, cores []cpuTimes, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return agg, nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		ct := parseCPUTimes(fields[1:])
		if fields[0] == "cpu" {
			agg = ct
		} else {
			cores = append(cores, ct)
		}
	}
	return agg, cores, sc.Err()
}

func parseCPUTimes(f []string) cpuTimes {
	get := func(i int) float64 {
		if i < len(f) {
			return parseFloat(f[i])
		}
		return 0
	}
	return cpuTimes{
		user: get(0), nice: get(1), system: get(2), idle: get(3),
		iowait: get(4), irq: get(5), softirq: get(6), steal: get(7),
	}
}

func readLoadAvg() [3]float64 {
	var la [3]float64
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return la
	}
	f := strings.Fields(string(data))
	for i := 0; i < 3 && i < len(f); i++ {
		la[i] = parseFloat(f[i])
	}
	return la
}

func readMemInfo() MemInfo {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemInfo{}
	}
	defer f.Close()

	var totalKB, availKB int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			totalKB = atoi(fields[1])
		case "MemAvailable:":
			availKB = atoi(fields[1])
		}
	}
	return buildMemInfo(totalKB*1024, availKB*1024)
}

// --- macOS (degraded: no per-core) ---

func collectSystemDarwin() (*SysData, error) {
	sd := &SysData{}
	// Preferred: real per-core ticks via host_processor_info (cgo builds only).
	if agg, cores, ok := darwinCPUTimes(); ok {
		computeCPUUsage(sd, agg, cores)
	} else {
		// Fallback (CGO_ENABLED=0 builds): aggregate only, no per-core.
		sd.NumCPU = runtime.NumCPU()
		sd.CPUTotal, sd.User, sd.System, sd.Idle = darwinCPU()
	}
	sd.LoadAvg = darwinLoadAvg()
	sd.Mem = darwinMem()
	sd.Procs = topProcs("ps", "-Ac", "-o", "pid,user,pcpu,pmem,comm", "-r")
	return sd, nil
}

// darwinCPU parses the "CPU usage: X% user, Y% sys, Z% idle" line from top -l 1.
func darwinCPU() (busy, user, system, idle float64) {
	out, err := exec.Command("top", "-l", "1", "-n", "0").Output()
	if err != nil {
		return 0, 0, 0, 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "CPU usage:") {
			continue
		}
		rest := strings.TrimPrefix(line, "CPU usage:")
		for _, part := range strings.Split(rest, ",") {
			f := strings.Fields(strings.TrimSpace(part))
			if len(f) < 2 {
				continue
			}
			v := parseFloat(strings.TrimSuffix(f[0], "%"))
			switch f[1] {
			case "user":
				user = v
			case "sys":
				system = v
			case "idle":
				idle = v
			}
		}
		break
	}
	return clampPct(user + system), user, system, idle
}

func darwinLoadAvg() [3]float64 {
	var la [3]float64
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return la
	}
	// Format: { 1.23 2.34 3.45 }
	f := strings.Fields(strings.Trim(strings.TrimSpace(string(out)), "{}"))
	for i := 0; i < 3 && i < len(f); i++ {
		la[i] = parseFloat(f[i])
	}
	return la
}

func darwinMem() MemInfo {
	var total int
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		total = atoi(strings.TrimSpace(string(out)))
	}
	// vm_stat reports page counts; free ≈ free + inactive + speculative pages.
	out, err := exec.Command("vm_stat").Output()
	if err != nil || total == 0 {
		return buildMemInfo(total, 0)
	}
	pageSize := 4096
	var freePages, inactive, speculative int
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "page size of") {
			for _, tok := range strings.Fields(line) {
				if n := atoi(tok); n > 0 {
					pageSize = n
				}
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		v := atoi(strings.TrimSpace(strings.TrimSuffix(parts[1], ".")))
		switch strings.TrimSpace(parts[0]) {
		case "Pages free":
			freePages = v
		case "Pages inactive":
			inactive = v
		case "Pages speculative":
			speculative = v
		}
	}
	avail := (freePages + inactive + speculative) * pageSize
	return buildMemInfo(total, avail)
}

// --- shared helpers ---

func buildMemInfo(totalBytes, availBytes int) MemInfo {
	if totalBytes <= 0 {
		return MemInfo{}
	}
	used := totalBytes - availBytes
	if used < 0 {
		used = 0
	}
	return MemInfo{
		TotalMB:     totalBytes / 1024 / 1024,
		UsedMB:      used / 1024 / 1024,
		AvailableMB: availBytes / 1024 / 1024,
		UsedPct:     float64(used) / float64(totalBytes) * 100,
	}
}

// topProcs runs a ps command and returns the union of the top 15 by CPU and the
// top 15 by memory (deduped by PID). Expected column order: pid user pcpu pmem comm.
func topProcs(name string, args ...string) []ProcEntry {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil
	}

	var all []ProcEntry
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		all = append(all, ProcEntry{
			PID:     atoi(fields[0]),
			User:    fields[1],
			CPU:     parseFloat(fields[2]),
			Mem:     parseFloat(fields[3]),
			Command: strings.Join(fields[4:], " "),
		})
	}

	const topN = 5
	seen := make(map[int]struct{})
	var result []ProcEntry

	add := func(list []ProcEntry) {
		for i := 0; i < len(list) && i < topN; i++ {
			if _, ok := seen[list[i].PID]; ok {
				continue
			}
			seen[list[i].PID] = struct{}{}
			result = append(result, list[i])
		}
	}

	byCPU := append([]ProcEntry(nil), all...)
	sort.Slice(byCPU, func(i, j int) bool { return byCPU[i].CPU > byCPU[j].CPU })
	add(byCPU)

	byMem := append([]ProcEntry(nil), all...)
	sort.Slice(byMem, func(i, j int) bool { return byMem[i].Mem > byMem[j].Mem })
	add(byMem)

	return result
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}
