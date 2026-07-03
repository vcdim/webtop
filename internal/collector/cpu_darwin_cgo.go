//go:build darwin && cgo

package collector

/*
#include <mach/mach.h>
#include <mach/processor_info.h>
#include <mach/mach_host.h>
*/
import "C"

import "unsafe"

// darwinCPUTimes reads per-CPU tick counters via host_processor_info and returns
// the aggregate plus per-core cpuTimes. Only built with cgo enabled (native
// `go build` on macOS); the CGO_ENABLED=0 release build uses the stub instead.
func darwinCPUTimes() (agg cpuTimes, cores []cpuTimes, ok bool) {
	var ncpu C.natural_t
	var info C.processor_info_array_t
	var infoCnt C.mach_msg_type_number_t

	kr := C.host_processor_info(
		C.host_t(C.mach_host_self()),
		C.PROCESSOR_CPU_LOAD_INFO,
		&ncpu,
		&info,
		&infoCnt,
	)
	if kr != C.KERN_SUCCESS || info == nil {
		return cpuTimes{}, nil, false
	}
	// The array is owned by the kernel; release it when done.
	defer C.vm_deallocate(
		C.vm_map_t(C.mach_task_self_),
		C.vm_address_t(uintptr(unsafe.Pointer(info))),
		C.vm_size_t(infoCnt)*C.vm_size_t(unsafe.Sizeof(C.integer_t(0))),
	)

	n := int(ncpu)
	const stateMax = C.CPU_STATE_MAX
	ticks := (*[1 << 28]C.integer_t)(unsafe.Pointer(info))[: n*stateMax : n*stateMax]

	cores = make([]cpuTimes, n)
	for i := 0; i < n; i++ {
		base := i * stateMax
		ct := cpuTimes{
			user:   float64(ticks[base+C.CPU_STATE_USER]),
			system: float64(ticks[base+C.CPU_STATE_SYSTEM]),
			idle:   float64(ticks[base+C.CPU_STATE_IDLE]),
			nice:   float64(ticks[base+C.CPU_STATE_NICE]),
		}
		cores[i] = ct
		agg.user += ct.user
		agg.system += ct.system
		agg.idle += ct.idle
		agg.nice += ct.nice
	}
	return agg, cores, true
}
