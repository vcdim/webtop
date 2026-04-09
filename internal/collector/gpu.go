package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type GPUInfo struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	GPUUtil  int    `json:"gpu_util"`
	MemUsed  int    `json:"mem_used"`
	MemTotal int    `json:"mem_total"`
}

type GPUProcess struct {
	GPUIndex  int    `json:"gpu_index"`
	PID       int    `json:"pid"`
	Name      string `json:"name"`
	User      string `json:"user"`
	MemUsedMB int    `json:"mem_used_mb"`
}

type GPUData struct {
	GPUs      []GPUInfo    `json:"gpus"`
	Processes []GPUProcess `json:"processes"`
}

func CollectGPU() (*GPUData, error) {
	gpus, err := queryGPUs()
	if err != nil {
		return nil, err
	}
	if len(gpus) == 0 {
		return &GPUData{}, nil
	}

	procs, _ := queryGPUProcesses(gpus)

	return &GPUData{
		GPUs:      gpus,
		Processes: procs,
	}, nil
}

func queryGPUs() ([]GPUInfo, error) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi failed: %w", err)
	}

	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 5 {
			continue
		}
		gpus = append(gpus, GPUInfo{
			Index:    atoi(fields[0]),
			Name:     strings.TrimSpace(fields[1]),
			GPUUtil:  atoi(fields[2]),
			MemUsed:  atoi(fields[3]),
			MemTotal: atoi(fields[4]),
		})
	}
	return gpus, nil
}

func queryGPUProcesses(gpus []GPUInfo) ([]GPUProcess, error) {
	out, err := exec.Command("nvidia-smi",
		"--query-compute-apps=gpu_uuid,pid,process_name,used_gpu_memory",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil, err
	}

	// Build UUID→index map
	uuidOut, err := exec.Command("nvidia-smi",
		"--query-gpu=index,uuid",
		"--format=csv,noheader",
	).Output()
	if err != nil {
		return nil, err
	}

	uuidMap := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(string(uuidOut)), "\n") {
		parts := strings.SplitN(line, ", ", 2)
		if len(parts) == 2 {
			uuidMap[strings.TrimSpace(parts[1])] = atoi(parts[0])
		}
	}

	var procs []GPUProcess
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 4 {
			continue
		}
		uuid := strings.TrimSpace(fields[0])
		pid := atoi(fields[1])
		procs = append(procs, GPUProcess{
			GPUIndex:  uuidMap[uuid],
			PID:       pid,
			Name:      strings.TrimSpace(fields[2]),
			User:      lookupUser(pid),
			MemUsedMB: atoi(fields[3]),
		})
	}
	return procs, nil
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	// Handle float values like "150.00" from power readings
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	v, _ := strconv.Atoi(s)
	return v
}
