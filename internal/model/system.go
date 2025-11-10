package model

// SystemStatus provides the struct to store the basic system status
type SystemStatus struct {
	Percent float64 `json:"percent"`
	Total   float64 `json:"total"`
	Used    float64 `json:"used"`
}
