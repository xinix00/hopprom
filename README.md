# hopprom - Hop Prometheus Metrics Exporter

Prometheus metrics exporter for Hop clusters. Polls the local hop agent API and exposes cluster metrics in Prometheus format.

## Features

- **Stateless** - No persistent storage, all metrics calculated from API
- **Poll-based** - Query hop API every 5 seconds (configurable)
- **KISS** - Pure Go stdlib, no dependencies
- **Counters** - Track task starts, failures, restarts with delta detection
- **Per-agent metrics** - CPU/memory capacity and usage

## Installation

```bash
cd hopprom
go build -o ../bin/hopprom ./cmd/hopprom
```

## Usage

```bash
# Start with defaults (polls localhost:8080, exposes :9090/metrics)
./bin/hopprom

# Custom configuration
./bin/hopprom \
  -listen :9090 \
  -agent http://127.0.0.1:8080 \
  -interval 5s
```

## Metrics

### Agent Metrics

```prometheus
hop_agents_total                         # Total registered agents
hop_agents_healthy                       # Healthy agents (seen <60s)
hop_agent_cpu_cores{agent="..."}         # CPU cores per agent
hop_agent_cpu_used_cores{agent="..."}    # Used CPU cores
hop_agent_memory_bytes{agent="..."}      # Total memory
hop_agent_memory_used_bytes{agent="..."} # Used memory
```

### Task Metrics

```prometheus
hop_tasks_total{state="running|failed|stopped"}  # Tasks by state
hop_tasks_running{job="..."}                     # Running tasks per job
hop_task_restarts{job="..."}                     # Current restart count
```

### Job Metrics

```prometheus
hop_jobs_total                           # Total jobs
hop_job_instances_running{job="..."}     # Running instances
hop_job_instances_expected{job="..."}    # Expected instances (count)
hop_job_healthy{job="..."}               # 1 if running >= expected, 0 otherwise
```

### Counters (monotonic)

```prometheus
hop_task_starts_total{job="..."}      # Total task starts detected
hop_task_failures_total{job="..."}    # Total failures (state -> failed)
hop_task_restarts_total{job="..."}    # Total restarts (restartCount increases)
```

## Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'hop'
    scrape_interval: 10s
    static_configs:
      - targets: ['node1:9090', 'node2:9090', 'node3:9090']
```

## Alerting Examples

```yaml
# alerts.yml
groups:
  - name: hop
    rules:
      # Job degraded - not enough instances running
      - alert: HopJobDegraded
        expr: hop_job_instances_running < hop_job_instances_expected
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Job {{ $labels.job }} is degraded ({{ $value }} / {{ $expected }} instances)"

      # Agent down - cluster has too few healthy agents
      - alert: HopAgentDown
        expr: hop_agents_healthy < 3
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Only {{ $value }} healthy agents (expected 3+)"

      # High failure rate
      - alert: HopHighFailureRate
        expr: rate(hop_task_failures_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Job {{ $labels.job }} has high failure rate ({{ $value }}/s)"

      # Agent overutilized
      - alert: HopAgentCPUHigh
        expr: |
          (hop_agent_cpu_used_cores / hop_agent_cpu_cores) > 0.9
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Agent {{ $labels.agent }} CPU usage high ({{ $value | humanizePercentage }})"
```

## Deploy with Hop

Run hopprom with `count: 1` (agent proxies cluster-wide endpoints to leader):

```json
{
  "name": "hopprom",
  "command": "/usr/local/bin/hopprom -listen :9090 -agent http://127.0.0.1:8080",
  "count": 1,
  "ports": {"metrics": 9090},
  "tags": {
    "urlprefix": "urlprefix:metrics.*"
  }
}
```

**Why count=1?** Agent proxies `/v1/*` requests to leader, so one instance gets cluster-wide data. If node fails, hop reschedules automatically.

## Architecture

```
┌─────────────┐
│  hop    │ :8080
│  (agent)    │
└──────┬──────┘
       │ poll every 5s
       │ GET /v1/agents
       │ GET /v1/jobs
       │ GET /v1/status
       │ GET /capacity (per agent)
       ↓
┌──────────────┐
│  hopprom    │ :9090
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
- **Poll-based**: Same pattern as hopdns and hoplb (KISS)
- **No persistence**: If hopprom restarts, counters reset (acceptable for monitoring)
- **Per-node**: Each agent runs its own hopprom instance
- **Prometheus scrapes all**: Aggregate metrics across cluster in PromQL

## Example Queries

```promql
# Total tasks across cluster
sum(hop_tasks_total)

# Job health by instance
hop_job_healthy == 0

# Failure rate per job (5min)
rate(hop_task_failures_total[5m])

# Average restarts per job
avg(hop_task_restarts) by (job)

# Cluster CPU utilization
sum(hop_agent_cpu_used_cores) / sum(hop_agent_cpu_cores)

# Memory pressure per agent
(hop_agent_memory_used_bytes / hop_agent_memory_bytes) * 100
```
