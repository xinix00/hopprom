# easyprom - EasyRun Prometheus Metrics Exporter

Prometheus metrics exporter for EasyRun clusters. Polls the local easyrun agent API and exposes cluster metrics in Prometheus format.

## Features

- **Stateless** - No persistent storage, all metrics calculated from API
- **Poll-based** - Query easyrun API every 5 seconds (configurable)
- **KISS** - Pure Go stdlib, no dependencies
- **Counters** - Track task starts, failures, restarts with delta detection
- **Per-agent metrics** - CPU/memory capacity and usage

## Installation

```bash
cd easyprom
go build -o ../bin/easyprom ./cmd/easyprom
```

## Usage

```bash
# Start with defaults (polls localhost:8080, exposes :9090/metrics)
./bin/easyprom

# Custom configuration
./bin/easyprom \
  -listen :9090 \
  -agent http://127.0.0.1:8080 \
  -interval 5s
```

## Metrics

### Agent Metrics

```prometheus
easyrun_agents_total                         # Total registered agents
easyrun_agents_healthy                       # Healthy agents (seen <60s)
easyrun_agent_cpu_cores{agent="..."}         # CPU cores per agent
easyrun_agent_cpu_used_cores{agent="..."}    # Used CPU cores
easyrun_agent_memory_bytes{agent="..."}      # Total memory
easyrun_agent_memory_used_bytes{agent="..."} # Used memory
```

### Task Metrics

```prometheus
easyrun_tasks_total{state="running|failed|stopped"}  # Tasks by state
easyrun_tasks_running{job="..."}                     # Running tasks per job
easyrun_task_restarts{job="..."}                     # Current restart count
```

### Job Metrics

```prometheus
easyrun_jobs_total                           # Total jobs
easyrun_job_instances_running{job="..."}     # Running instances
easyrun_job_instances_expected{job="..."}    # Expected instances (count)
easyrun_job_healthy{job="..."}               # 1 if running >= expected, 0 otherwise
```

### Counters (monotonic)

```prometheus
easyrun_task_starts_total{job="..."}      # Total task starts detected
easyrun_task_failures_total{job="..."}    # Total failures (state -> failed)
easyrun_task_restarts_total{job="..."}    # Total restarts (restartCount increases)
```

## Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'easyrun'
    scrape_interval: 10s
    static_configs:
      - targets: ['node1:9090', 'node2:9090', 'node3:9090']
```

## Alerting Examples

```yaml
# alerts.yml
groups:
  - name: easyrun
    rules:
      # Job degraded - not enough instances running
      - alert: EasyRunJobDegraded
        expr: easyrun_job_instances_running < easyrun_job_instances_expected
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Job {{ $labels.job }} is degraded ({{ $value }} / {{ $expected }} instances)"

      # Agent down - cluster has too few healthy agents
      - alert: EasyRunAgentDown
        expr: easyrun_agents_healthy < 3
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Only {{ $value }} healthy agents (expected 3+)"

      # High failure rate
      - alert: EasyRunHighFailureRate
        expr: rate(easyrun_task_failures_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Job {{ $labels.job }} has high failure rate ({{ $value }}/s)"

      # Agent overutilized
      - alert: EasyRunAgentCPUHigh
        expr: |
          (easyrun_agent_cpu_used_cores / easyrun_agent_cpu_cores) > 0.9
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Agent {{ $labels.agent }} CPU usage high ({{ $value | humanizePercentage }})"
```

## Deploy with EasyRun

Run easyprom on all agents using `count: -1`:

```json
{
  "name": "easyprom",
  "command": "/usr/local/bin/easyprom -listen :9090 -agent http://127.0.0.1:8080",
  "count": -1,
  "ports": {"metrics": 9090},
  "tags": {
    "urlprefix": "urlprefix:metrics.*"
  }
}
```

## Architecture

```
┌─────────────┐
│  easyrun    │ :8080
│  (agent)    │
└──────┬──────┘
       │ poll every 5s
       │ GET /v1/agents
       │ GET /v1/jobs
       │ GET /v1/status
       │ GET /capacity (per agent)
       ↓
┌──────────────┐
│  easyprom    │ :9090
│              │
└──────┬───────┘
       │ scrape every 10s
       │ GET /metrics
       ↓
┌──────────────┐
│ Prometheus   │
└──────────────┘
```

## Design

- **Stateless**: Counters are recalculated on every scrape by tracking state transitions
- **Poll-based**: Same pattern as easydns and easylb (KISS)
- **No persistence**: If easyprom restarts, counters reset (acceptable for monitoring)
- **Per-node**: Each agent runs its own easyprom instance
- **Prometheus scrapes all**: Aggregate metrics across cluster in PromQL

## Example Queries

```promql
# Total tasks across cluster
sum(easyrun_tasks_total)

# Job health by instance
easyrun_job_healthy == 0

# Failure rate per job (5min)
rate(easyrun_task_failures_total[5m])

# Average restarts per job
avg(easyrun_task_restarts) by (job)

# Cluster CPU utilization
sum(easyrun_agent_cpu_used_cores) / sum(easyrun_agent_cpu_cores)

# Memory pressure per agent
(easyrun_agent_memory_used_bytes / easyrun_agent_memory_bytes) * 100
```
