package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	agentHealthyThreshold = 60 * time.Second // agent is healthy if seen in last 60s
	httpTimeout          = 5 * time.Second
)

// Collector polls easyrun API and calculates metrics
type Collector struct {
	agentURL string
	client   *http.Client
	apiKey   string

	mu             sync.RWMutex
	metrics        *Metrics
	lastTaskStates map[string]string // taskID -> state (for detecting transitions)
}

// New creates a new collector
func New(agentURL string, apiKey string) *Collector {
	return &Collector{
		agentURL:       agentURL,
		client:         &http.Client{Timeout: httpTimeout},
		apiKey:         apiKey,
		lastTaskStates: make(map[string]string),
		metrics: &Metrics{
			TasksByState:      make(map[string]int),
			TasksByJob:        make(map[string]int),
			TaskRestartsByJob: make(map[string]int),
			JobInstances:      make(map[string]*JobMetric),
			TaskStartsTotal:   make(map[string]int),
			TaskFailuresTotal: make(map[string]int),
			TaskRestartsTotal: make(map[string]int),
			AgentLastSeen:     make(map[string]time.Time),
			AgentCapacity:     make(map[string]*CapacityResponse),
			AgentCPUUsed:      make(map[string]float64),
			AgentMemoryUsed:   make(map[string]uint64),
		},
	}
}

// Collect fetches data from easyrun and calculates metrics
func (c *Collector) Collect() error {
	// Fetch data from APIs
	agents, err := c.fetchAgents()
	if err != nil {
		return fmt.Errorf("failed to fetch agents: %w", err)
	}

	jobs, err := c.fetchJobs()
	if err != nil {
		return fmt.Errorf("failed to fetch jobs: %w", err)
	}

	// Fetch task details per job (parallel, each only queries relevant agents)
	tasksByAgent := c.fetchAllJobTasks(jobs)

	// Fetch capacity for each agent (async)
	c.fetchAgentCapacities(agents)

	// Calculate metrics
	c.calculateMetrics(agents, jobs, tasksByAgent)

	return nil
}

// GetMetrics returns a copy of current metrics (thread-safe)
func (c *Collector) GetMetrics() *Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	m := &Metrics{
		AgentsTotal:        c.metrics.AgentsTotal,
		AgentsHealthy:      c.metrics.AgentsHealthy,
		JobsTotal:          c.metrics.JobsTotal,
		TasksByState:       make(map[string]int),
		TasksByJob:         make(map[string]int),
		TaskRestartsByJob:  make(map[string]int),
		JobInstances:       make(map[string]*JobMetric),
		TaskStartsTotal:    make(map[string]int),
		TaskFailuresTotal:  make(map[string]int),
		TaskRestartsTotal:  make(map[string]int),
		AgentLastSeen:      make(map[string]time.Time),
		AgentCapacity:      make(map[string]*CapacityResponse),
		AgentCPUUsed:       make(map[string]float64),
		AgentMemoryUsed:    make(map[string]uint64),
	}

	for k, v := range c.metrics.TasksByState {
		m.TasksByState[k] = v
	}
	for k, v := range c.metrics.TasksByJob {
		m.TasksByJob[k] = v
	}
	for k, v := range c.metrics.TaskRestartsByJob {
		m.TaskRestartsByJob[k] = v
	}
	for k, v := range c.metrics.JobInstances {
		m.JobInstances[k] = &JobMetric{
			Running:    v.Running,
			Expected:   v.Expected,
			Healthy:    v.Healthy,
			CPUPercent: v.CPUPercent,
			MemPercent: v.MemPercent,
		}
	}
	for k, v := range c.metrics.TaskStartsTotal {
		m.TaskStartsTotal[k] = v
	}
	for k, v := range c.metrics.TaskFailuresTotal {
		m.TaskFailuresTotal[k] = v
	}
	for k, v := range c.metrics.TaskRestartsTotal {
		m.TaskRestartsTotal[k] = v
	}
	for k, v := range c.metrics.AgentLastSeen {
		m.AgentLastSeen[k] = v
	}
	for k, v := range c.metrics.AgentCapacity {
		m.AgentCapacity[k] = v
	}
	for k, v := range c.metrics.AgentCPUUsed {
		m.AgentCPUUsed[k] = v
	}
	for k, v := range c.metrics.AgentMemoryUsed {
		m.AgentMemoryUsed[k] = v
	}

	return m
}

func (c *Collector) calculateMetrics(agents []*Agent, jobs []*Job, tasksByAgent map[string][]*Task) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset gauges (counters are never reset)
	c.metrics.TasksByState = make(map[string]int)
	c.metrics.TasksByJob = make(map[string]int)
	c.metrics.TaskRestartsByJob = make(map[string]int)
	c.metrics.JobInstances = make(map[string]*JobMetric)
	c.metrics.AgentCPUUsed = make(map[string]float64)
	c.metrics.AgentMemoryUsed = make(map[string]uint64)

	// Agent metrics
	c.metrics.AgentsTotal = len(agents)
	c.metrics.AgentsHealthy = 0
	for _, agent := range agents {
		c.metrics.AgentLastSeen[agent.ID] = agent.LastSeen
		if time.Since(agent.LastSeen) < agentHealthyThreshold {
			c.metrics.AgentsHealthy++
		}
	}

	// Build job lookup
	jobMap := make(map[string]*Job)
	for _, job := range jobs {
		jobMap[job.Name] = job
	}

	// Task metrics
	jobCPUSum := make(map[string]float64)  // jobName -> sum of CPU%
	jobMemSum := make(map[string]float64)  // jobName -> sum of Mem%
	jobRunCount := make(map[string]int)    // jobName -> running task count (for avg)

	for _, tasks := range tasksByAgent {
		for _, task := range tasks {
			// Count by state
			c.metrics.TasksByState[task.State]++

			// Count by job + accumulate resource usage
			if task.State == "running" {
				c.metrics.TasksByJob[task.JobName]++
				jobCPUSum[task.JobName] += task.CPUPercent
				jobMemSum[task.JobName] += task.MemPercent
				jobRunCount[task.JobName]++
			}

			// Count restarts
			if task.RestartCount > 0 {
				c.metrics.TaskRestartsByJob[task.JobName] += task.RestartCount
			}

			// Detect state transitions for counters
			oldState := c.lastTaskStates[task.ID]
			if oldState != task.State {
				// New task or state changed
				if oldState == "" {
					// New task started
					c.metrics.TaskStartsTotal[task.JobName]++
				} else if task.State == "failed" {
					// Task failed
					c.metrics.TaskFailuresTotal[task.JobName]++
				}

				// Track restarts (monotonic counter)
				if task.RestartCount > 0 {
					if prevTask, ok := c.lastTaskStates[task.ID]; ok && prevTask == "running" {
						c.metrics.TaskRestartsTotal[task.JobName]++
					}
				}

				c.lastTaskStates[task.ID] = task.State
			}
		}
	}

	// Agent resource usage: use agent-reported capacity (accurate, even after recovery)
	for agentID, cap := range c.metrics.AgentCapacity {
		c.metrics.AgentCPUUsed[agentID] = float64(cap.CPUUsedShares) / 1024.0
		c.metrics.AgentMemoryUsed[agentID] = cap.MemoryUsedBytes
	}

	// Job metrics
	c.metrics.JobsTotal = len(jobs)
	for _, job := range jobs {
		running := c.metrics.TasksByJob[job.Name]
		expected := job.Count
		if expected == -1 {
			expected = len(agents)
		} else if expected == 0 {
			expected = 1
		}

		var avgCPU, avgMem float64
		if cnt := jobRunCount[job.Name]; cnt > 0 {
			avgCPU = jobCPUSum[job.Name] / float64(cnt)
			avgMem = jobMemSum[job.Name] / float64(cnt)
		}

		c.metrics.JobInstances[job.Name] = &JobMetric{
			Running:    running,
			Expected:   expected,
			Healthy:    running >= expected,
			CPUPercent: avgCPU,
			MemPercent: avgMem,
		}
	}
}

// newRequest creates a GET request with API key authentication
func (c *Collector) newRequest(url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	return req, nil
}

func (c *Collector) fetchAgents() ([]*Agent, error) {
	req, err := c.newRequest(c.agentURL + "/v1/agents")
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var agents []*Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	return agents, nil
}

func (c *Collector) fetchJobs() ([]*Job, error) {
	req, err := c.newRequest(c.agentURL + "/v1/jobs")
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var jobs []*Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}

	return jobs, nil
}

// fetchAllJobTasks fetches task details for each job in parallel via per-job status.
// Each per-job query only contacts agents that have that job placed — much cheaper
// than the old GetClusterStatus which contacted ALL agents.
func (c *Collector) fetchAllJobTasks(jobs []*Job) map[string][]*Task {
	result := make(map[string][]*Task)
	if len(jobs) == 0 {
		return result
	}

	type jobResult struct {
		tasks map[string][]*Task
	}
	ch := make(chan jobResult, len(jobs))

	for _, job := range jobs {
		go func(j *Job) {
			tasks, err := c.fetchJobStatus(j.Name)
			if err != nil {
				log.Printf("Failed to fetch status for job %s: %v", j.Name, err)
				ch <- jobResult{}
				return
			}
			ch <- jobResult{tasks: tasks}
		}(job)
	}

	for range jobs {
		r := <-ch
		for agentID, tasks := range r.tasks {
			result[agentID] = append(result[agentID], tasks...)
		}
	}

	return result
}

func (c *Collector) fetchJobStatus(jobName string) (map[string][]*Task, error) {
	req, err := c.newRequest(fmt.Sprintf("%s/v1/jobs/%s/status", c.agentURL, jobName))
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var status JobStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return status.TasksByAgent, nil
}

func (c *Collector) fetchAgentCapacities(agents []*Agent) {
	type result struct {
		id  string
		cap *CapacityResponse
	}
	ch := make(chan result, len(agents))

	// Fetch all agents in parallel (always refresh — capacity includes live usage)
	for _, agent := range agents {
		go func(a *Agent) {
			cap, err := c.fetchAgentCapacity(a.Endpoint)
			if err != nil {
				log.Printf("Failed to fetch capacity for %s: %v", a.ID, err)
				ch <- result{a.ID, nil}
				return
			}
			ch <- result{a.ID, cap}
		}(agent)
	}

	// Collect all results
	c.mu.Lock()
	defer c.mu.Unlock()
	for range agents {
		r := <-ch
		if r.cap != nil {
			c.metrics.AgentCapacity[r.id] = r.cap
		}
	}
}

func (c *Collector) fetchAgentCapacity(endpoint string) (*CapacityResponse, error) {
	req, err := c.newRequest(endpoint + "/capacity")
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var cap CapacityResponse
	if err := json.NewDecoder(resp.Body).Decode(&cap); err != nil {
		return nil, err
	}

	return &cap, nil
}
