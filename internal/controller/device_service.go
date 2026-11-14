package controller

import (
	"sync"
	"time"

	"sound-signal-generator/internal/audio"
)

// DeviceChangeEvent indicates hardware device connection state changes.
type DeviceChangeEvent struct {
	Type          string             `json:"type"` // "plugged", "unplugged", "default_changed"
	DeviceID      string             `json:"device_id"`
	DeviceName    string             `json:"device_name"`
	Timestamp     time.Time          `json:"timestamp"`
	ActiveWarning string             `json:"active_warning,omitempty"`
}

// DeviceService manages device lifecycle, enumeration, and hotplug monitoring.
type DeviceService struct {
	mu            sync.Mutex
	backend       audio.AudioBackend
	lastDevices   map[string]audio.DeviceInfo
	activeDevID   string
	disconnected  bool
	changeHandler func(event DeviceChangeEvent)
	stopWatcher   chan struct{}
}

// NewDeviceService creates a device service with the specified audio backend.
func NewDeviceService(backend audio.AudioBackend) *DeviceService {
	return &DeviceService{
		backend:     backend,
		lastDevices: make(map[string]audio.DeviceInfo),
	}
}

// SetChangeHandler sets an optional callback triggered when hardware changes.
func (ds *DeviceService) SetChangeHandler(fn func(event DeviceChangeEvent)) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.changeHandler = fn
}

// SetActiveDeviceID sets the currently active device ID to monitor for disconnection.
func (ds *DeviceService) SetActiveDeviceID(id string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.activeDevID = id
	ds.disconnected = false
}

// IsActiveDeviceDisconnected checks if the active device was disconnected.
func (ds *DeviceService) IsActiveDeviceDisconnected() bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.disconnected
}

// Enumerate gets the current list of devices and updates cache.
func (ds *DeviceService) Enumerate() ([]audio.DeviceInfo, error) {
	devices, err := ds.backend.EnumerateDevices()
	if err != nil {
		return nil, err
	}

	ds.mu.Lock()
	newMap := make(map[string]audio.DeviceInfo)
	for _, d := range devices {
		newMap[d.ID] = d
	}
	ds.lastDevices = newMap
	ds.mu.Unlock()

	return devices, nil
}

// CheckHotplug performs an instantaneous check against cached device list.
// Returns an event if a change occurred, or nil.
func (ds *DeviceService) CheckHotplug() *DeviceChangeEvent {
	currentDevs, err := ds.backend.EnumerateDevices()
	if err != nil {
		return nil
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()

	currentMap := make(map[string]audio.DeviceInfo)
	for _, d := range currentDevs {
		currentMap[d.ID] = d
	}

	var event *DeviceChangeEvent

	// 1. Check if active device unplugged
	if ds.activeDevID != "" {
		if _, exists := currentMap[ds.activeDevID]; !exists && !ds.disconnected {
			ds.disconnected = true
			oldInfo := ds.lastDevices[ds.activeDevID]
			name := oldInfo.Name
			if name == "" {
				name = ds.activeDevID
			}
			event = &DeviceChangeEvent{
				Type:          "unplugged",
				DeviceID:      ds.activeDevID,
				DeviceName:    name,
				Timestamp:     time.Now().UTC(),
				ActiveWarning: "当前音频输出设备已被物理拔出或断开连接！输出已自动进入安全故障静音状态。",
			}
		} else if _, exists := currentMap[ds.activeDevID]; exists && ds.disconnected {
			// Reconnected
			ds.disconnected = false
			event = &DeviceChangeEvent{
				Type:       "plugged",
				DeviceID:   ds.activeDevID,
				DeviceName: currentMap[ds.activeDevID].Name,
				Timestamp:  time.Now().UTC(),
			}
		}
	}

	ds.lastDevices = currentMap
	if event != nil && ds.changeHandler != nil {
		go ds.changeHandler(*event)
	}

	return event
}

// StartWatcher starts a background goroutine polling every 250ms (< 500ms latency requirement).
func (ds *DeviceService) StartWatcher() {
	ds.mu.Lock()
	if ds.stopWatcher != nil {
		ds.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	ds.stopWatcher = stop
	ds.mu.Unlock()

	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ds.CheckHotplug()
			}
		}
	}()
}

// StopWatcher terminates the background polling watcher.
func (ds *DeviceService) StopWatcher() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.stopWatcher != nil {
		close(ds.stopWatcher)
		ds.stopWatcher = nil
	}
}
