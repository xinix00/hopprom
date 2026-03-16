package collector

import "time"

// CapacityResponse from easyrun agent /capacity endpoint
type CapacityResponse struct {
	CPUCores        int    `json:"cpu_cores"`
	MemoryBytes     uint64 `json:"memory_bytes"`
	CPUUsedShares   int    `json:"cpu_used_shares"`
	MemoryUsedBytes uint64 `json:"memory_used_bytes"`
	TasksRunning    int    `json:"tasks_running"`
}

// Metrics holds calculated Prometheus metrics
type Metrics struct {
	AgentsTotal        int
	AgentsHealthy      int
	AgentLastSeen      map[string]time.Time
	AgentCapacity      map[string]*CapacityResponse
	AgentCPUUsed       map[string]float64
	AgentMemoryUsed    map[string]uint64
	TasksByState       map[string]int
	TasksByJob         map[string]int
	TaskRestartsByJob  map[string]int
	JobsTotal          int
	JobInstances       map[string]*JobMetric
	TaskStartsTotal    map[string]int
	TaskFailuresTotal  map[string]int
	TaskRestartsTotal  map[string]int
}

type JobMetric struct {
	Running    int
	Expected   int
	Healthy    bool
	CPUPercent float64
	MemPercent float64
}
