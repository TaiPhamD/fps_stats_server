//go:build windows
// +build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
)

const (
	FILE_MAP_READ      = 0x0004
	MAHM_SHARED_MEMORY = "MAHMSharedMemory"
)

// structures based on MAHM v2.0 spec
type SharedHeader struct {
	Signature     uint32
	Version       uint32
	HeaderSize    uint32
	NumEntries    uint32
	EntrySize     uint32
	Time          int64
	NumGpuEntries uint32
	GpuEntrySize  uint32
}

type Entry struct {
	SrcName   [260]byte
	SrcUnits  [260]byte
	LocalName [260]byte
	LocalUnit [260]byte
	Format    [260]byte
	Data      float32
	Min, Max  float32
	Flags     uint32
	GpuIndex  uint32
	SrcId     uint32
	_pad      uint32 // alignment padding
}

// API Response structures
type SensorData struct {
	Name      string   `json:"name"`
	Value     *float32 `json:"value,omitempty"`
	Unit      string   `json:"unit"`
	GpuIndex  uint32   `json:"gpu_index"`
	Category  string   `json:"category"`
	Timestamp int64    `json:"timestamp"`
}

type SystemStatus struct {
	Timestamp   int64           `json:"timestamp"`
	FPS         [100]SensorData `json:"fps"`
	GPU         [100]SensorData `json:"gpu"`
	CPU         [100]SensorData `json:"cpu"`
	Memory      [100]SensorData `json:"memory"`
	All         [100]SensorData `json:"all"`
	FPSCount    int             `json:"fps_count"`
	GPUCount    int             `json:"gpu_count"`
	CPUCount    int             `json:"cpu_count"`
	MemoryCount int             `json:"memory_count"`
	AllCount    int             `json:"all_count"`
}

type MemInfo struct {
	HeapAllocMB   uint64 `json:"heap_alloc_mb"`
	TotalAllocMB  uint64 `json:"total_alloc_mb"`
	HeapInuseMB   uint64 `json:"heap_inuse_mb"`
	NumGoroutines int    `json:"num_goroutines"`
	NumGC         uint32 `json:"num_gc"`
	Timestamp     int64  `json:"timestamp"`
}

type FPSApp struct {
	stop       chan bool
	port       string
	latestData SystemStatus
	dataMutex  chan struct{}
}

func main() {
	// Kill any existing fps_tray.exe process
	killExistingProcess()

	app := &FPSApp{
		stop:      make(chan bool),
		port:      "8080",
		dataMutex: make(chan struct{}, 1),
	}

	// Start the data collection goroutine
	go app.collectData()

	// Start the HTTP server
	go app.startServer()

	// Run the system tray
	systray.Run(app.onReady, app.onExit)
}

func (app *FPSApp) onReady() {
	// Set custom icon
	systray.SetIcon(getIcon())
	systray.SetTitle("FPS Monitor")
	systray.SetTooltip("FPS Monitoring Server")

	// Add menu items
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu events
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			case <-app.stop:
				return
			}
		}
	}()
}

func (app *FPSApp) onExit() {
	// Signal all goroutines to stop
	close(app.stop)
}

func killExistingProcess() {
	// Get current process ID
	currentPID := os.Getpid()

	// Use tasklist to find fps_tray.exe processes
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq fps_tray.exe", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return // Ignore errors, just continue
	}

	// Parse the output to find PIDs
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "fps_tray.exe") {
			// Extract PID from CSV format: "fps_tray.exe","1234","Console","1","1,234 K"
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], "\"")
				if pid, err := strconv.Atoi(pidStr); err == nil {
					// Don't kill ourselves
					if pid != currentPID {
						fmt.Printf("Killing existing fps_tray.exe process (PID: %d)\n", pid)
						// Use taskkill to terminate the process
						killCmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
						killCmd.Run() // Ignore errors
					}
				}
			}
		}
	}

	// Give a moment for the process to terminate
	time.Sleep(100 * time.Millisecond)
}

func getIcon() []byte {
	// Read the icon file
	iconData, err := os.ReadFile("FPSserver.ico")
	if err != nil {
		// Return empty icon if file not found
		return []byte{}
	}
	return iconData
}

func (app *FPSApp) collectData() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Read data directly into the existing structure
			select {
			case app.dataMutex <- struct{}{}:
				app.readMSIDataInto(&app.latestData)
				app.latestData.Timestamp = time.Now().Unix()
				<-app.dataMutex
			default:
				// Skip if mutex is busy
			}
		case <-app.stop:
			return
		}
	}
}

func (app *FPSApp) readMSIDataInto(status *SystemStatus) {
	// Use a static buffer to avoid allocations
	nameBytes := []byte(MAHM_SHARED_MEMORY + "\x00")

	// Load DLL functions
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openFileMapping := kernel32.NewProc("OpenFileMappingA")
	closeHandle := kernel32.NewProc("CloseHandle")
	mapViewOfFile := kernel32.NewProc("MapViewOfFile")
	unmapViewOfFile := kernel32.NewProc("UnmapViewOfFile")

	handle, _, _ := openFileMapping.Call(uintptr(FILE_MAP_READ), 0, uintptr(unsafe.Pointer(&nameBytes[0])))
	if handle == 0 {
		// Reset counters
		status.FPSCount = 0
		status.GPUCount = 0
		status.CPUCount = 0
		status.MemoryCount = 0
		status.AllCount = 0
		return
	}
	defer closeHandle.Call(handle)

	view, _, _ := mapViewOfFile.Call(handle, uintptr(FILE_MAP_READ), 0, 0, 0)
	if view == 0 {
		// Reset counters
		status.FPSCount = 0
		status.GPUCount = 0
		status.CPUCount = 0
		status.MemoryCount = 0
		status.AllCount = 0
		return
	}
	defer unmapViewOfFile.Call(view)

	hdr := (*SharedHeader)(unsafe.Pointer(view))
	if hdr == nil || hdr.Signature != 0x4D41484D { // 'MAHM' in little-endian
		// Reset counters
		status.FPSCount = 0
		status.GPUCount = 0
		status.CPUCount = 0
		status.MemoryCount = 0
		status.AllCount = 0
		return
	}

	entriesBase := view + uintptr(hdr.HeaderSize)

	// Reset counters
	status.FPSCount = 0
	status.GPUCount = 0
	status.CPUCount = 0
	status.MemoryCount = 0
	status.AllCount = 0

	for i := uint32(0); i < hdr.NumEntries; i++ {
		off := uintptr(i) * uintptr(hdr.EntrySize)
		e := (*Entry)(unsafe.Pointer(entriesBase + off))

		// Validate entry pointer
		if e == nil {
			continue
		}

		// Find null terminators safely
		nameEnd := bytes.IndexByte(e.SrcName[:], 0)
		if nameEnd == -1 {
			nameEnd = len(e.SrcName)
		}
		unitEnd := bytes.IndexByte(e.SrcUnits[:], 0)
		if unitEnd == -1 {
			unitEnd = len(e.SrcUnits)
		}

		// Convert to strings
		nameStr := string(e.SrcName[:nameEnd])
		unitStr := string(e.SrcUnits[:unitEnd])

		// Handle invalid values for HomeAssistant compatibility
		var sensorValue *float32
		if e.Data >= 3.4e+38 || e.Data <= -3.4e+38 {
			sensorValue = nil // Use null for invalid values
		} else {
			// Create a copy of the data to avoid memory issues
			dataCopy := e.Data
			sensorValue = &dataCopy
		}

		sensor := SensorData{
			Name:      nameStr,
			Value:     sensorValue,
			Unit:      unitStr,
			GpuIndex:  e.GpuIndex,
			Category:  app.categorizeSensor(nameStr),
			Timestamp: status.Timestamp,
		}

		// Add to All array if space available
		if status.AllCount < 100 {
			status.All[status.AllCount] = sensor
			status.AllCount++
		}

		// Categorize sensors
		switch sensor.Category {
		case "fps":
			if status.FPSCount < 100 {
				status.FPS[status.FPSCount] = sensor
				status.FPSCount++
			}
		case "gpu":
			if status.GPUCount < 100 {
				status.GPU[status.GPUCount] = sensor
				status.GPUCount++
			}
		case "cpu":
			if status.CPUCount < 100 {
				status.CPU[status.CPUCount] = sensor
				status.CPUCount++
			}
		case "memory":
			if status.MemoryCount < 100 {
				status.Memory[status.MemoryCount] = sensor
				status.MemoryCount++
			}
		}
	}
}

func (app *FPSApp) categorizeSensor(name string) string {
	nameLower := bytes.ToLower([]byte(name))

	if bytes.Contains(nameLower, []byte("fps")) || bytes.Contains(nameLower, []byte("framerate")) || bytes.Contains(nameLower, []byte("frametime")) {
		return "fps"
	}
	if bytes.Contains(nameLower, []byte("gpu")) {
		return "gpu"
	}
	if bytes.Contains(nameLower, []byte("cpu")) {
		return "cpu"
	}
	if bytes.Contains(nameLower, []byte("memory")) || bytes.Contains(nameLower, []byte("ram")) {
		return "memory"
	}
	return "other"
}

func (app *FPSApp) startServer() {
	// HTTP handlers
	http.HandleFunc("/cpu", app.handleCPU)
	http.HandleFunc("/fps", app.handleFPS)
	http.HandleFunc("/gpu", app.handleGPU)
	http.HandleFunc("/memory", app.handleMemory)
	http.HandleFunc("/debug/memory", app.handleMemoryStats)
	http.HandleFunc("/", app.handleRoot)

	_ = http.ListenAndServe("0.0.0.0:"+app.port, nil)
}

// Static response for root
var rootResponse = map[string]interface{}{
	"service": "FPS Monitor",
	"version": "1.0.0",
	"endpoints": map[string]string{
		"cpu":    "/cpu",
		"fps":    "/fps",
		"gpu":    "/gpu",
		"memory": "/memory",
	},
}

func (app *FPSApp) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(rootResponse)
}

func (app *FPSApp) handleFPS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	select {
	case app.dataMutex <- struct{}{}:
		defer func() { <-app.dataMutex }()
		json.NewEncoder(w).Encode(app.latestData.FPS[:app.latestData.FPSCount])
	default:
		http.Error(w, "Data not available", http.StatusServiceUnavailable)
	}
}

func (app *FPSApp) handleGPU(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	select {
	case app.dataMutex <- struct{}{}:
		defer func() { <-app.dataMutex }()
		json.NewEncoder(w).Encode(app.latestData.GPU[:app.latestData.GPUCount])
	default:
		http.Error(w, "Data not available", http.StatusServiceUnavailable)
	}
}

func (app *FPSApp) handleCPU(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	select {
	case app.dataMutex <- struct{}{}:
		defer func() { <-app.dataMutex }()
		json.NewEncoder(w).Encode(app.latestData.CPU[:app.latestData.CPUCount])
	default:
		http.Error(w, "Data not available", http.StatusServiceUnavailable)
	}
}

func (app *FPSApp) handleMemory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	select {
	case app.dataMutex <- struct{}{}:
		defer func() { <-app.dataMutex }()
		json.NewEncoder(w).Encode(app.latestData.Memory[:app.latestData.MemoryCount])
	default:
		http.Error(w, "Data not available", http.StatusServiceUnavailable)
	}
}

func (app *FPSApp) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	info := MemInfo{
		HeapAllocMB:   m.HeapAlloc / 1024 / 1024,
		TotalAllocMB:  m.TotalAlloc / 1024 / 1024,
		HeapInuseMB:   m.HeapInuse / 1024 / 1024,
		NumGoroutines: runtime.NumGoroutine(),
		NumGC:         m.NumGC,
		Timestamp:     time.Now().Unix(),
	}

	json.NewEncoder(w).Encode(info)
}
