package model

import "time"

// Memory provides a basic struct to describe memory status
type Memory struct {
	Total             float64   `json:"total"`
	Used              float64   `json:"used"`
	Self              float64   `json:"self"`
	TotalAllocated    float64   `json:"totalAllocated"`
	CurrentAllocated  float64   `json:"currentAllocated"`
	Malloc            uint64    `json:"malloc"`
	Free              uint64    `json:"free"`
	Heap              float64   `json:"heap"`
	NumGC             uint32    `json:"numGC"`
	PauseTotalSec     float64   `json:"pauseTotalSec"`
	LastGC            time.Time `json:"lastGC"`
	LastPauseTotalSec float64   `json:"lastPauseTotalSec"`
	NextGC            float64   `json:"nextGC"`
	Lookups           uint64    `json:"lookups"`
}

// SystemStatus provides the struct to store the basic system status
type SystemStatus struct {
	Percent float64 `json:"percent"`
	Memory  Memory  `json:"memory"`
}
