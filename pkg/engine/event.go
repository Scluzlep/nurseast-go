package engine

import (
	"encoding/json"
	"sync"
	"time"
)

type EventType string

const (
	EventLog             EventType = "log"
	EventOverallProgress EventType = "progress"
	EventVideoProgress   EventType = "video_progress"
	EventStatusChange    EventType = "status"
)

type Event struct {
	Type      EventType   `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type LogPayload struct {
	Level   string `json:"level"` // "info", "success", "warn", "error"
	Message string `json:"message"`
}

type OverallProgressPayload struct {
	Current      int    `json:"current"`
	Total        int    `json:"total"`
	Percentage   int    `json:"percentage"`
	CurrentTitle string `json:"current_title"`
}

type VideoProgressPayload struct {
	VID            int64  `json:"vid"`
	Title          string `json:"title"`
	CurrentSeconds int64  `json:"current_seconds"`
	TotalSeconds   int64  `json:"total_seconds"`
	Percentage     int    `json:"percentage"`
}

type StatusPayload struct {
	Status string `json:"status"` // "idle", "running", "paused", "stopping", "stopped", "completed", "failed"
}

// EventHub 负责向所有 SSE 客户端广播事件
type EventHub struct {
	subscribers map[chan []byte]struct{}
	mu          sync.RWMutex
}

var Hub = &EventHub{
	subscribers: make(map[chan []byte]struct{}),
}

func (h *EventHub) Subscribe() chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan []byte, 64)
	h.subscribers[ch] = struct{}{}
	return ch
}

func (h *EventHub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers, ch)
	close(ch)
}

func (h *EventHub) Publish(e Event) {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().Format("15:04:05")
	}
	bytes, err := json.Marshal(e)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- bytes:
		default:
		}
	}
}

func EmitLog(level, message string) {
	Hub.Publish(Event{
		Type: EventLog,
		Data: LogPayload{Level: level, Message: message},
	})
}
