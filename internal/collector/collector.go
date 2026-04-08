package collector

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type PortEntry struct {
	Port    int    `json:"port"`
	Proto   string `json:"proto"`
	Process string `json:"process"`
	Command string `json:"command"`
	PID     int    `json:"pid"`
	User    string `json:"user"`
	Address string `json:"address"`
	State   string `json:"state"`
	Count   int    `json:"count"`
}

var processRegex = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

func Collect() ([]PortEntry, error) {
	if runtime.GOOS == "darwin" {
		return collectDarwin()
	}
	out, err := exec.Command("ss", "-tulnp").Output()
	if err != nil {
		return nil, fmt.Errorf("ss command failed: %w", err)
	}
	return parse(string(out))
}

// collectDarwin uses lsof to gather port info on macOS (for development/testing).
func collectDarwin() ([]PortEntry, error) {
	out, err := exec.Command("lsof", "+c", "0", "-iTCP", "-iUDP", "-sTCP:LISTEN", "-nP").Output()
	if err != nil {
		return nil, fmt.Errorf("lsof command failed: %w", err)
	}
	return parseLsof(string(out))
}

func parseLsof(output string) ([]PortEntry, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	type dedupKey struct {
		Port  int
		PID   int
		Proto string
	}
	seen := make(map[dedupKey]*PortEntry)

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		procName := strings.ReplaceAll(fields[0], `\x20`, " ")
		pid, _ := strconv.Atoi(fields[1])
		username := fields[2]
		proto := strings.ToLower(fields[7])
		if strings.Contains(proto, "tcp") {
			proto = "tcp"
		} else if strings.Contains(proto, "udp") {
			proto = "udp"
		}
		nameField := fields[8]
		addr, portStr := splitAddrPort(nameField)
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}
		state := "LISTEN"
		if len(fields) > 9 {
			state = strings.Trim(fields[9], "()")
		}

		key := dedupKey{Port: port, PID: pid, Proto: proto}
		if entry, ok := seen[key]; ok {
			entry.Count++
		} else {
			seen[key] = &PortEntry{
				Port:    port,
				Proto:   proto,
				Process: procName,
				PID:     pid,
				User:    username,
				Address: addr,
				State:   state,
				Count:   1,
			}
		}
	}

	entries := make([]PortEntry, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		return entries[i].Proto < entries[j].Proto
	})
	return entries, nil
}

func parse(output string) ([]PortEntry, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	type dedupKey struct {
		Port  int
		PID   int
		Proto string
	}
	seen := make(map[dedupKey]*PortEntry)

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		state := fields[0]
		localAddr := fields[4]

		proto := "tcp"
		if strings.HasPrefix(line, "udp") {
			proto = "udp"
		}

		// For UDP, state field may be "UNCONN" which is at fields[0],
		// and the layout is the same as TCP.
		// Parse local address:port
		addr, portStr := splitAddrPort(localAddr)
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// Extract process info
		var procName string
		var pid int
		match := processRegex.FindStringSubmatch(line)
		if match != nil {
			procName = match[1]
			pid, _ = strconv.Atoi(match[2])
			// ss uses the 15-char comm name; read full name from /proc
			if fullName := readProcName(pid); fullName != "" {
				procName = fullName
			}
		}

		// Look up username from PID
		username := lookupUser(pid)

		key := dedupKey{Port: port, PID: pid, Proto: proto}
		if entry, ok := seen[key]; ok {
			entry.Count++
		} else {
			seen[key] = &PortEntry{
				Port:    port,
				Proto:   proto,
				Process: procName,
				Command: readCmdline(pid),
				PID:     pid,
				User:    username,
				Address: addr,
				State:   state,
				Count:   1,
			}
		}
	}

	entries := make([]PortEntry, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		return entries[i].Proto < entries[j].Proto
	})
	return entries, nil
}

// readCmdline reads the full command line from /proc/<pid>/cmdline.
func readCmdline(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return ""
	}
	// cmdline is null-separated; join with spaces
	return strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " ")
}

// readProcName reads the full executable name from /proc/<pid>/exe or /proc/<pid>/cmdline.
func readProcName(pid int) string {
	if pid <= 0 {
		return ""
	}
	// Try reading the exe symlink first
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err == nil {
		return filepath.Base(exe)
	}
	// Fallback to cmdline
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return ""
	}
	// cmdline is null-separated; first element is the command
	cmd := strings.SplitN(string(data), "\x00", 2)[0]
	return filepath.Base(cmd)
}

func splitAddrPort(s string) (string, string) {
	// Handle IPv6 [::]:port
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func lookupUser(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := exec.Command("stat", "-c", "%U", fmt.Sprintf("/proc/%d", pid)).Output()
	if err != nil {
		// Fallback: try reading /proc/pid/status
		return lookupUserFromProc(pid)
	}
	return strings.TrimSpace(string(data))
}

func lookupUserFromProc(pid int) string {
	data, err := exec.Command("cat", fmt.Sprintf("/proc/%d/status", pid)).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				u, err := user.LookupId(fields[1])
				if err == nil {
					return u.Username
				}
				return fields[1]
			}
		}
	}
	return ""
}
