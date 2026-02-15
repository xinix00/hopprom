package exporter

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"easyprom/internal/collector"
)

// Exporter exposes metrics in Prometheus format
type Exporter struct {
	collector *collector.Collector
}

// New creates a new Prometheus exporter
func New(c *collector.Collector) *Exporter {
	return &Exporter{
		collector: c,
	}
}

// ServeHTTP handles /metrics requests
func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := e.collector.GetMetrics()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	var b strings.Builder

	// Agent metrics
	b.WriteString("# HELP easyrun_agents_total Total number of registered agents\n")
	b.WriteString("# TYPE easyrun_agents_total gauge\n")
	fmt.Fprintf(&b, "easyrun_agents_total %d\n", metrics.AgentsTotal)
	b.WriteString("\n")

	b.WriteString("# HELP easyrun_agents_healthy Number of healthy agents (seen in last 60s)\n")
	b.WriteString("# TYPE easyrun_agents_healthy gauge\n")
	fmt.Fprintf(&b, "easyrun_agents_healthy %d\n", metrics.AgentsHealthy)
	b.WriteString("\n")

	// Agent capacity metrics
	if len(metrics.AgentCapacity) > 0 {
		b.WriteString("# HELP easyrun_agent_cpu_cores Total CPU cores per agent\n")
		b.WriteString("# TYPE easyrun_agent_cpu_cores gauge\n")
		for agentID, cap := range metrics.AgentCapacity {
			fmt.Fprintf(&b, "easyrun_agent_cpu_cores{agent=%q} %d\n", agentID, cap.CPUCores)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_agent_cpu_used_cores Used CPU cores per agent\n")
		b.WriteString("# TYPE easyrun_agent_cpu_used_cores gauge\n")
		for agentID, used := range metrics.AgentCPUUsed {
			fmt.Fprintf(&b, "easyrun_agent_cpu_used_cores{agent=%q} %.2f\n", agentID, used)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_agent_memory_bytes Total memory bytes per agent\n")
		b.WriteString("# TYPE easyrun_agent_memory_bytes gauge\n")
		for agentID, cap := range metrics.AgentCapacity {
			fmt.Fprintf(&b, "easyrun_agent_memory_bytes{agent=%q} %d\n", agentID, cap.MemoryBytes)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_agent_memory_used_bytes Used memory bytes per agent\n")
		b.WriteString("# TYPE easyrun_agent_memory_used_bytes gauge\n")
		for agentID, used := range metrics.AgentMemoryUsed {
			fmt.Fprintf(&b, "easyrun_agent_memory_used_bytes{agent=%q} %d\n", agentID, used)
		}
		b.WriteString("\n")
	}

	// Task metrics by state
	b.WriteString("# HELP easyrun_tasks_total Number of tasks by state\n")
	b.WriteString("# TYPE easyrun_tasks_total gauge\n")
	for state, count := range metrics.TasksByState {
		fmt.Fprintf(&b, "easyrun_tasks_total{state=%q} %d\n", state, count)
	}
	b.WriteString("\n")

	// Task metrics by job
	if len(metrics.TasksByJob) > 0 {
		b.WriteString("# HELP easyrun_tasks_running Number of running tasks per job\n")
		b.WriteString("# TYPE easyrun_tasks_running gauge\n")
		for job, count := range metrics.TasksByJob {
			fmt.Fprintf(&b, "easyrun_tasks_running{job=%q} %d\n", job, count)
		}
		b.WriteString("\n")
	}

	// Task restart metrics
	if len(metrics.TaskRestartsByJob) > 0 {
		b.WriteString("# HELP easyrun_task_restarts Current restart count per job\n")
		b.WriteString("# TYPE easyrun_task_restarts gauge\n")
		for job, count := range metrics.TaskRestartsByJob {
			fmt.Fprintf(&b, "easyrun_task_restarts{job=%q} %d\n", job, count)
		}
		b.WriteString("\n")
	}

	// Job metrics
	b.WriteString("# HELP easyrun_jobs_total Total number of jobs\n")
	b.WriteString("# TYPE easyrun_jobs_total gauge\n")
	fmt.Fprintf(&b, "easyrun_jobs_total %d\n", metrics.JobsTotal)
	b.WriteString("\n")

	// Job instances
	if len(metrics.JobInstances) > 0 {
		b.WriteString("# HELP easyrun_job_instances_running Running instances per job\n")
		b.WriteString("# TYPE easyrun_job_instances_running gauge\n")
		// Sort for consistent output
		jobs := make([]string, 0, len(metrics.JobInstances))
		for job := range metrics.JobInstances {
			jobs = append(jobs, job)
		}
		sort.Strings(jobs)
		for _, job := range jobs {
			m := metrics.JobInstances[job]
			fmt.Fprintf(&b, "easyrun_job_instances_running{job=%q} %d\n", job, m.Running)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_job_instances_expected Expected instances per job\n")
		b.WriteString("# TYPE easyrun_job_instances_expected gauge\n")
		for _, job := range jobs {
			m := metrics.JobInstances[job]
			fmt.Fprintf(&b, "easyrun_job_instances_expected{job=%q} %d\n", job, m.Expected)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_job_healthy Job is healthy (running >= expected)\n")
		b.WriteString("# TYPE easyrun_job_healthy gauge\n")
		for _, job := range jobs {
			m := metrics.JobInstances[job]
			health := 0
			if m.Healthy {
				health = 1
			}
			fmt.Fprintf(&b, "easyrun_job_healthy{job=%q} %d\n", job, health)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_job_cpu_percent Average CPU usage percent per job (relative to allocation)\n")
		b.WriteString("# TYPE easyrun_job_cpu_percent gauge\n")
		for _, job := range jobs {
			m := metrics.JobInstances[job]
			fmt.Fprintf(&b, "easyrun_job_cpu_percent{job=%q} %.1f\n", job, m.CPUPercent)
		}
		b.WriteString("\n")

		b.WriteString("# HELP easyrun_job_mem_percent Average memory usage percent per job (relative to allocation)\n")
		b.WriteString("# TYPE easyrun_job_mem_percent gauge\n")
		for _, job := range jobs {
			m := metrics.JobInstances[job]
			fmt.Fprintf(&b, "easyrun_job_mem_percent{job=%q} %.1f\n", job, m.MemPercent)
		}
		b.WriteString("\n")
	}

	// Counters (monotonic)
	if len(metrics.TaskStartsTotal) > 0 {
		b.WriteString("# HELP easyrun_task_starts_total Total task starts per job\n")
		b.WriteString("# TYPE easyrun_task_starts_total counter\n")
		jobs := make([]string, 0, len(metrics.TaskStartsTotal))
		for job := range metrics.TaskStartsTotal {
			jobs = append(jobs, job)
		}
		sort.Strings(jobs)
		for _, job := range jobs {
			count := metrics.TaskStartsTotal[job]
			fmt.Fprintf(&b, "easyrun_task_starts_total{job=%q} %d\n", job, count)
		}
		b.WriteString("\n")
	}

	if len(metrics.TaskFailuresTotal) > 0 {
		b.WriteString("# HELP easyrun_task_failures_total Total task failures per job\n")
		b.WriteString("# TYPE easyrun_task_failures_total counter\n")
		jobs := make([]string, 0, len(metrics.TaskFailuresTotal))
		for job := range metrics.TaskFailuresTotal {
			jobs = append(jobs, job)
		}
		sort.Strings(jobs)
		for _, job := range jobs {
			count := metrics.TaskFailuresTotal[job]
			fmt.Fprintf(&b, "easyrun_task_failures_total{job=%q} %d\n", job, count)
		}
		b.WriteString("\n")
	}

	if len(metrics.TaskRestartsTotal) > 0 {
		b.WriteString("# HELP easyrun_task_restarts_total Total task restarts per job\n")
		b.WriteString("# TYPE easyrun_task_restarts_total counter\n")
		jobs := make([]string, 0, len(metrics.TaskRestartsTotal))
		for job := range metrics.TaskRestartsTotal {
			jobs = append(jobs, job)
		}
		sort.Strings(jobs)
		for _, job := range jobs {
			count := metrics.TaskRestartsTotal[job]
			fmt.Fprintf(&b, "easyrun_task_restarts_total{job=%q} %d\n", job, count)
		}
		b.WriteString("\n")
	}

	w.Write([]byte(b.String()))
}
