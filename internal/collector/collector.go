package collector

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type PortEntry struct {
	Port    int    `json:"port"`
	Proto   string `json:"proto"`
	Command string `json:"command"`
	PID     int    `json:"pid"`
	User    string `json:"user"`
	Address string `json:"address"`
	Count   int    `json:"count"`
}

type dedupKey struct {
	Port  int
	PID   int
	Proto string
}

var processRegex = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

func Collect() ([]PortEntry, error) {
	if runtime.GOOS == "darwin" {
		return collectDarwin()
	}
	return collectLinux()
}

func collectLinux() ([]PortEntry, error) {
	out, err := exec.Command("ss", "-tulnp").Output()
	if err != nil {
		return nil, fmt.Errorf("ss command failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	seen := make(map[dedupKey]*PortEntry)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		proto := "tcp"
		if strings.HasPrefix(line, "udp") {
			proto = "udp"
		}

		addr, portStr := splitAddrPort(fields[4])
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		var pid int
		match := processRegex.FindStringSubmatch(line)
		if match != nil {
			pid, _ = strconv.Atoi(match[2])
		}

		key := dedupKey{Port: port, PID: pid, Proto: proto}
		if entry, ok := seen[key]; ok {
			entry.Count++
		} else {
			seen[key] = &PortEntry{
				Port:    port,
				Proto:   proto,
				Command: readCmdline(pid),
				PID:     pid,
				User:    lookupUser(pid),
				Address: addr,
				Count:   1,
			}
		}
	}

	return sortEntries(seen), nil
}

func collectDarwin() ([]PortEntry, error) {
	out, err := exec.Command("lsof", "+c", "0", "-iTCP", "-iUDP", "-sTCP:LISTEN", "-nP").Output()
	if err != nil {
		return nil, fmt.Errorf("lsof command failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	seen := make(map[dedupKey]*PortEntry)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		command := strings.ReplaceAll(fields[0], `\x20`, " ")
		pid, _ := strconv.Atoi(fields[1])
		username := fields[2]

		proto := strings.ToLower(fields[7])
		if strings.Contains(proto, "tcp") {
			proto = "tcp"
		} else if strings.Contains(proto, "udp") {
			proto = "udp"
		}

		addr, portStr := splitAddrPort(fields[8])
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		key := dedupKey{Port: port, PID: pid, Proto: proto}
		if entry, ok := seen[key]; ok {
			entry.Count++
		} else {
			seen[key] = &PortEntry{
				Port:    port,
				Proto:   proto,
				Command: command,
				PID:     pid,
				User:    username,
				Address: addr,
				Count:   1,
			}
		}
	}

	return sortEntries(seen), nil
}

func sortEntries(seen map[dedupKey]*PortEntry) []PortEntry {
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
	return entries
}

func splitAddrPort(s string) (string, string) {
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func readCmdline(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return ""
	}
	return strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " ")
}

func lookupUser(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if u, err := user.LookupId(fields[1]); err == nil {
					return u.Username
				}
				return fields[1]
			}
		}
	}
	return ""
}
