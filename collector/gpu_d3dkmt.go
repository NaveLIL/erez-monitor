//go:build windows

package collector

import (
	"syscall"
	"unsafe"
)

var (
	gdi32DLL                      = syscall.NewLazyDLL("gdi32.dll")
	procD3DKMTEnumAdapters2       = gdi32DLL.NewProc("D3DKMTEnumAdapters2")
	procD3DKMTQueryAdapterInfo    = gdi32DLL.NewProc("D3DKMTQueryAdapterInfo")
	procD3DKMTOpenAdapterFromLuid = gdi32DLL.NewProc("D3DKMTOpenAdapterFromLuid")
	procD3DKMTCloseAdapter        = gdi32DLL.NewProc("D3DKMTCloseAdapter")
	procD3DKMTQueryStatistics     = gdi32DLL.NewProc("D3DKMTQueryStatistics")
)

// D3DKMT constants
const (
	KMTQUERYADAPTERINFOTYPE_PERFDATA = 62 // KMTQAITYPE_ADAPTERPERFDATA
)

// LUID structure
type LUID struct {
	LowPart  uint32
	HighPart int32
}

// D3DKMT_ENUMADAPTERS2 structure
type D3DKMT_ENUMADAPTERS2 struct {
	NumAdapters uint32
	Padding     uint32
	Adapters    uintptr // Pointer to array of D3DKMT_ADAPTERINFO
}

// D3DKMT_ADAPTERINFO structure
type D3DKMT_ADAPTERINFO struct {
	AdapterHandle uint32
	AdapterLuid   LUID
	NumSources    uint32
	Present       uint32
}

// D3DKMT_OPENADAPTERFROMLUID structure
type D3DKMT_OPENADAPTERFROMLUID struct {
	AdapterLuid   LUID
	AdapterHandle uint32
}

// D3DKMT_CLOSEADAPTER structure
type D3DKMT_CLOSEADAPTER struct {
	AdapterHandle uint32
}

// D3DKMT_QUERYADAPTERINFO structure
type D3DKMT_QUERYADAPTERINFO struct {
	AdapterHandle   uint32
	Type            uint32
	PrivateData     uintptr
	PrivateDataSize uint32
}

// D3DKMT_ADAPTER_PERFDATA structure - contains temperature!
type D3DKMT_ADAPTER_PERFDATA struct {
	PhysicalAdapterIndex uint32
	MemoryFrequency      uint64
	MaxMemoryFrequency   uint64
	MaxMemoryFrequencyOC uint64
	MemoryBandwidth      uint64
	PCIEBandwidth        uint64
	FanRPM               uint32
	Power                uint32 // Power in milliwatts
	Temperature          uint32 // Temperature in deci-Celsius (divide by 10)
	PowerStateOverride   uint8
	Padding              [3]uint8
}

// GetGPUTemperatureD3DKMT gets GPU temperature using D3DKMT API (same as Task Manager)
func GetGPUTemperatureD3DKMT() (float64, error) {
	// First enumerate adapters
	var enumAdapters D3DKMT_ENUMADAPTERS2
	enumAdapters.NumAdapters = 0
	enumAdapters.Adapters = 0

	// First call to get count
	ret, _, _ := procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&enumAdapters)))
	if ret != 0 {
		return 0, syscall.Errno(ret)
	}

	if enumAdapters.NumAdapters == 0 {
		return 0, syscall.EINVAL
	}

	// Allocate array for adapters
	adapters := make([]D3DKMT_ADAPTERINFO, enumAdapters.NumAdapters)
	enumAdapters.Adapters = uintptr(unsafe.Pointer(&adapters[0]))

	// Second call to get adapters
	ret, _, _ = procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&enumAdapters)))
	if ret != 0 {
		return 0, syscall.Errno(ret)
	}

	// Try each adapter
	for i := uint32(0); i < enumAdapters.NumAdapters; i++ {
		adapter := adapters[i]

		// Open adapter by LUID
		var openAdapter D3DKMT_OPENADAPTERFROMLUID
		openAdapter.AdapterLuid = adapter.AdapterLuid

		ret, _, _ = procD3DKMTOpenAdapterFromLuid.Call(uintptr(unsafe.Pointer(&openAdapter)))
		if ret != 0 {
			continue
		}

		// Query performance data (includes temperature)
		var perfData D3DKMT_ADAPTER_PERFDATA
		perfData.PhysicalAdapterIndex = 0

		var queryInfo D3DKMT_QUERYADAPTERINFO
		queryInfo.AdapterHandle = openAdapter.AdapterHandle
		queryInfo.Type = KMTQUERYADAPTERINFOTYPE_PERFDATA
		queryInfo.PrivateData = uintptr(unsafe.Pointer(&perfData))
		queryInfo.PrivateDataSize = uint32(unsafe.Sizeof(perfData))

		ret, _, _ = procD3DKMTQueryAdapterInfo.Call(uintptr(unsafe.Pointer(&queryInfo)))

		// Close adapter
		var closeAdapter D3DKMT_CLOSEADAPTER
		closeAdapter.AdapterHandle = openAdapter.AdapterHandle
		procD3DKMTCloseAdapter.Call(uintptr(unsafe.Pointer(&closeAdapter)))

		if ret == 0 && perfData.Temperature > 0 {
			// Temperature is in deci-Celsius, convert to Celsius
			temp := float64(perfData.Temperature) / 10.0
			if temp > 0 && temp < 150 {
				return temp, nil
			}
		}
	}

	return 0, syscall.EINVAL
}

// GetGPUPerfDataD3DKMT gets GPU performance data including temperature, power, fan
func GetGPUPerfDataD3DKMT() (temperature float64, powerWatts float64, fanRPM uint32, err error) {
	var enumAdapters D3DKMT_ENUMADAPTERS2
	enumAdapters.NumAdapters = 0
	enumAdapters.Adapters = 0

	ret, _, _ := procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&enumAdapters)))
	if ret != 0 {
		return 0, 0, 0, syscall.Errno(ret)
	}

	if enumAdapters.NumAdapters == 0 {
		return 0, 0, 0, syscall.EINVAL
	}

	adapters := make([]D3DKMT_ADAPTERINFO, enumAdapters.NumAdapters)
	enumAdapters.Adapters = uintptr(unsafe.Pointer(&adapters[0]))

	ret, _, _ = procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&enumAdapters)))
	if ret != 0 {
		return 0, 0, 0, syscall.Errno(ret)
	}

	for i := uint32(0); i < enumAdapters.NumAdapters; i++ {
		adapter := adapters[i]

		var openAdapter D3DKMT_OPENADAPTERFROMLUID
		openAdapter.AdapterLuid = adapter.AdapterLuid

		ret, _, _ = procD3DKMTOpenAdapterFromLuid.Call(uintptr(unsafe.Pointer(&openAdapter)))
		if ret != 0 {
			continue
		}

		var perfData D3DKMT_ADAPTER_PERFDATA
		perfData.PhysicalAdapterIndex = 0

		var queryInfo D3DKMT_QUERYADAPTERINFO
		queryInfo.AdapterHandle = openAdapter.AdapterHandle
		queryInfo.Type = KMTQUERYADAPTERINFOTYPE_PERFDATA
		queryInfo.PrivateData = uintptr(unsafe.Pointer(&perfData))
		queryInfo.PrivateDataSize = uint32(unsafe.Sizeof(perfData))

		ret, _, _ = procD3DKMTQueryAdapterInfo.Call(uintptr(unsafe.Pointer(&queryInfo)))

		var closeAdapter D3DKMT_CLOSEADAPTER
		closeAdapter.AdapterHandle = openAdapter.AdapterHandle
		procD3DKMTCloseAdapter.Call(uintptr(unsafe.Pointer(&closeAdapter)))

		if ret == 0 && perfData.Temperature > 0 {
			temp := float64(perfData.Temperature) / 10.0
			power := float64(perfData.Power) / 1000.0 // milliwatts to watts
			if temp > 0 && temp < 150 {
				return temp, power, perfData.FanRPM, nil
			}
		}
	}

	return 0, 0, 0, syscall.EINVAL
}
