//go:build !darwin || !cgo

package collector

// darwinCPUTimes is unavailable without cgo (Linux, or macOS built with
// CGO_ENABLED=0). Returning ok=false makes the darwin path fall back to the
// aggregate-only `top` reading and the Linux path never calls it.
func darwinCPUTimes() (cpuTimes, []cpuTimes, bool) {
	return cpuTimes{}, nil, false
}
