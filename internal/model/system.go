package model

import "time"

// Memory provides a basic struct to describe memory status
type Memory struct {
	Total             float64   `json:"total"`
	Used              float64   `json:"used"`
	Self              float64   `json:"self"`
	TotalAllocated    float64   `json:"total_allocated"`
	CurrentAllocated  float64   `json:"current_allocated"`
	Malloc            uint64    `json:"malloc"`
	Free              uint64    `json:"free"`
	Heap              float64   `json:"heap"`
	NumGC             uint32    `json:"num_gc"`
	PauseTotalSec     float64   `json:"pause_total_sec"`
	LastGC            time.Time `json:"last_gc"`
	LastPauseTotalSec float64   `json:"last_pause_total_sec"`
	NextGC            float64   `json:"next_gc"`
	Lookups           uint64    `json:"lookups"`
}

// SystemStatus provides the struct to store the basic system status
type SystemStatus struct {
	Percent float64 `json:"percent"`
	Memory  Memory  `json:"memory"`
}
