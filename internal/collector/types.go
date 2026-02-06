package collector

import "time"

// API response types matching easyrun

type Agent struct {
	ID       string    `json:"id"`
	Endpoint string    `json:"endpoint"`
	LastSeen time.Time `json:"last_seen"`
}

type Job struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Count       int               `json:"count"`
	CPUShares   int               `json:"cpu_shares,omitempty"`
	MemoryLimit uint64            `json:"memory_limit,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

type Task struct {
	ID           string    `json:"id"`
	JobName      string    `json:"job_name"`
	State        string    `json:"state"` // "running", "failed", "stopped"
	Pid          int       `json:"pid"`
	RestartCount int       `json:"restart_count"`
	StartedAt    time.Time `json:"started_at"`
}

type StatusResponse struct {
	Agents       int                  `json:"agents"`
	TotalTasks   int                  `json:"total_tasks"`
	RunningTasks int                  `json:"running_tasks"`
	TasksByAgent map[string][]*Task   `json:"tasks_by_agent"`
}

type CapacityResponse struct {
	CPUCores        int    `json:"cpu_cores"`
	MemoryBytes     uint64 `json:"memory_bytes"`
	CPUUsedShares   int    `json:"cpu_used_shares"`
	MemoryUsedBytes uint64 `json:"memory_used_bytes"`
	TasksRunning    int    `json:"tasks_running"`
}

// Metrics holds calculated Prometheus metrics
type Metrics struct {
	// Agent metrics
	AgentsTotal        int
	AgentsHealthy      int
	AgentLastSeen      map[string]time.Time // agentID -> last_seen

	// Agent capacity
	AgentCapacity      map[string]*CapacityResponse // agentID -> capacity
	AgentCPUUsed       map[string]float64           // agentID -> used cores
	AgentMemoryUsed    map[string]uint64            // agentID -> used bytes

	// Task metrics
	TasksByState       map[string]int    // state -> count
	TasksByJob         map[string]int    // jobName -> running count
	TaskRestartsByJob  map[string]int    // jobName -> total restarts

	// Job metrics
	JobsTotal          int
	JobInstances       map[string]*JobMetric // jobName -> metrics

	// Counters (stateful)
	TaskStartsTotal    map[string]int    // jobName -> total starts
	TaskFailuresTotal  map[string]int    // jobName -> total failures
	TaskRestartsTotal  map[string]int    // jobName -> total restarts
}

type JobMetric struct {
	Running  int
	Expected int
	Healthy  bool
}
