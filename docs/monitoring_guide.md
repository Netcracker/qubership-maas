# MaaS OOB Dashboards Guide

## Overview

**Dashboard Name:** Messaging as a Service (MaaS)
**UID:** `cd1b7bbb-7179-40b8-8fe1-dccac6950dd4`
**Location:** `helm-templates/maas-service/templates/GrafanaDashboard.yaml`
**Default Time Range:** Last 24 hours
**Scrape Interval:** Every 10 seconds (configured in `PodMonitor.yaml`)

### Purpose

Single-pane-of-glass for the health and performance of a MaaS service pod. Covers:
- Service availability and DR state
- CPU and memory resource consumption
- Go runtime internals (goroutines, GC, heap)
- HTTP traffic success rates and latency
- PostgreSQL database activity and connection pool health
- Kafka and RabbitMQ broker connectivity

All panels are scoped to a single pod at a time using the top-of-page dropdowns.

---

## Template Variables (Dropdowns)

| Variable           | What it selects |
|--------------------|---|
| `datasource`       | The Prometheus / VictoriaMetrics data source |
| `namespace`        | Kubernetes namespace |
| `service_name`     | Container name (default: `maas-service`) |
| `pod_name`         | Specific pod replica |
| `samplin`          | Interval used in `rate()` calculations |
| `kafka_instances`  | Multi-select: which Kafka broker(s) to display |
| `rabbit_instances` | Multi-select: which RabbitMQ broker(s) to display |

---

## Row 0 — Status Bar (always visible)

> **When to use:** First row to check during any incident. Gives an instant health snapshot.

| Panel | Metric | Values & Colors | Purpose |
|---|---|---|---|
| **Health** | `kube_pod_container_status_running` | `1` = OK (green), `0` = FATAL (red), no data = N/A (orange) | Is the pod running right now? |
| **Uptime** | `time() - process_start_time_seconds` | Duration in seconds | How long the pod has been up. A very low value means a recent restart. |
| **DR Mode** | `maas_db_dr_mode` | `0` = Active (green), `1` = Standby (blue), `2` = Disabled (orange) | Current DR role. Standby pods do not serve traffic. |
| **Master DB Availability** | `maas_health_is_master_db_alive` | `1` = OK (green), `0` = UNAVAILABLE (dark-red) | Can the service reach the primary PostgreSQL? Red means all write operations are failing. |
| **Open FD Count** | `process_open_fds` | Raw count | Current open file descriptors. Spikes may indicate connection or file handle leaks. |
| **Go Routines Count** | `go_goroutines` | Raw count | Active goroutines. Sustained growth indicates a goroutine leak. |

---

## Row 1 — Process Overview

> **When to use:** Investigating CPU throttling, memory pressure, or OS-level resource exhaustion.

### CPU Usage

- **Metrics:** `rate(container_cpu_usage_seconds_total[$inter])` · `kube_pod_container_resource_limits_cpu_cores` · `kube_pod_container_resource_requests_cpu_cores`
- **Unit:** Millicores
- **What it shows:** Actual CPU usage (green fill) vs. the Kubernetes resource request (orange) and limit (red dashed line).
- **When to act:** If the green line consistently touches the red dashed limit, the pod is CPU-throttled. Either increase the CPU limit or optimize hot code paths.

### Committed Memory

- **Metrics:** `kube_pod_container_resource_limits_memory_bytes` · `container_memory_max_usage_bytes`
- **What it shows:** Memory limit vs. observed peak usage over time.
- **When to act:** If peak usage approaches the limit, the pod risks OOMKill. Increase the memory limit or investigate a heap leak (see Row 2).

### Memory (pod)

- **Metrics:** `process_resident_memory_bytes` · `process_virtual_memory_bytes`
- **Unit:** Bytes (logarithmic scale)
- **What it shows:** Resident (RSS) vs. virtual memory from the Go process perspective.
- **When to act:** Steady RSS growth over hours/days is the primary signal of a memory leak.

### Open Files

- **Metrics:** `process_max_fds` · `process_open_fds`
- **What it shows:** OS file descriptor limit vs. open count over time.
- **When to act:** If open count approaches the OS maximum, the process will start failing to open new connections or files. Restart the pod and investigate the source of unclosed handles.

### Go Routines Count

- **Metrics:** `go_goroutines` · `go_threads`
- **What it shows:** Goroutine and OS thread count over time.
- **When to act:** A slow, steady climb that never drops back to baseline indicates a goroutine leak. Use profiling (`/debug/pprof/goroutine`) to find the source.

---

## Row 2 — Go Process Memory Details

> **When to use:** Deep memory investigation — distinguishing heap pressure from GC issues or allocator overhead.

### Heap

- **Metrics:** `go_memstats_heap_sys_bytes` · `go_memstats_heap_inuse_bytes`
- **What it shows:** Memory reserved from the OS for the heap vs. the portion currently in use. A large gap between them is normal (GC freed objects but hasn't returned memory to OS yet). A growing `inuse` line is a leak signal.

### Off-Heap

- **Metrics:** `go_memstats_gc_sys_bytes` · `go_memstats_mspan_sys_bytes` · `go_memstats_mcache_sys_bytes` · `go_memstats_buck_hash_sys_bytes` · `go_memstats_other_sys_bytes` · `go_memstats_stack_sys_bytes`
- **What it shows:** Go runtime internal bookkeeping memory (GC metadata, span tables, stacks, etc.). These are not heap allocations. Normally small and stable.

### Allocated Objects Count

- **Metric:** `go_memstats_heap_objects`
- **What it shows:** Number of live objects on the heap.
- **When to act:** If this grows unboundedly, a cache or collection is accumulating entries without being evicted.

### Freed Objects Count

- **Metric:** `rate(go_memstats_frees_total[$__interval])`
- **What it shows:** Rate at which the GC is freeing objects.
- **When to act:** A drop to near-zero while the app is active and memory is growing indicates GC is not running — this is unusual and worth investigating with profiling.

### GC Duration

- **Metric:** `rate(go_gc_duration_seconds_sum[$__interval])`
- **What it shows:** Time per second spent in garbage collection.
- **When to act:** High values cause stop-the-world pauses, which appear as latency spikes in the HTTP Statistics panels. Reduce allocation rate or tune `GOGC`.

---

## Row 3 — HTTP Statistics

> **When to use:** Investigating API errors, latency complaints, or traffic pattern changes.

### Serving Requests Rate

- **Metrics:**
  - `sum(rate(http_requests_total{status_code!~"5.."}[$__interval]))` — labeled `success`
  - `sum(rate(http_requests_total{status_code=~"5.."}[$__interval]))` — labeled `failures`
- **What it shows:** Requests per second split into successful (non-5xx) and failed (5xx).
- **When to act:**
  - Failure line rises while success stays flat → service is generating internal errors.
  - Both lines drop → callers have stopped sending traffic or the service is unreachable.
  - Success drops while failure rises → service is degrading under load.

### Request Serving Time (non 500)

- **Metric:** `sum by(le)(rate(http_request_duration_seconds_bucket{status_code!~"5.."}[$__interval]))`
- **Visualization:** Heatmap
- **Histogram buckets:** 10ms, 100ms, 1s, 10s
- **What it shows:** Distribution of response latencies for successful requests over time. Dark orange = concentration of requests at that latency bucket.
- **When to act:** If the heatmap shifts rightward (toward 1s–10s), request processing is slowing down. Correlate with Database Insights (slow queries) or GC Duration (GC pauses). 5xx responses are excluded so fast rejections do not skew the picture.

---

## Row 4 — Database Insights

> **When to use:** Investigating slow responses caused by database bottlenecks or connection exhaustion.

### Connections Pool

- **Metrics:** `go_sql_stats_connections_max_open` (limit, red dashed) · `go_sql_stats_connections_open` · `go_sql_stats_connections_in_use`
- **What it shows:** How saturated the DB connection pool is.
- **When to act:** If `in_use` consistently equals `open` and both approach `limit`, new requests will queue up waiting for a free connection. This manifests as latency visible in the next panel.

### Wait Time for Available Connection from Pool

- **Metric:** `rate(go_sql_stats_connections_blocked_seconds[$__rate_interval])`
- **Unit:** Seconds/second
- **What it shows:** Time per second the application spent blocked waiting for a free DB connection.
- **When to act:** Any non-zero sustained value here means connection pool saturation is already causing request latency. Increase pool size or reduce query duration.

### Master SQL Requests Rate

- **Metric:** `rate(maas_db_request_count{type="master"}[$__rate_interval])` (split by `result=success/error`)
- **What it shows:** Rate of SQL requests to the primary (read-write) database.
- **When to act:** If error rate rises, write operations are failing. Cross-check with the **Master DB Availability** stat at the top. Check PostgreSQL logs for constraint violations, timeouts, or connectivity issues.

### Replica SQL Requests Rate

- **Metric:** `rate(maas_db_request_count{type="replica"}[$__rate_interval])` (split by `result=success/error`)
- **What it shows:** Rate of SQL requests to the read replica.
- **When to act:** If this drops to zero while the master rate is healthy, the replica may be down or unreachable. The app may have fallen back to routing all reads to the master, increasing its load.

### SQL Requests Execution Time

- **Metric:** `rate(maas_db_sql_executions_bucket[$__interval])`
- **Visualization:** Histogram
- **Histogram buckets:** 50ms, 200ms, 500ms, 1s
- **What it shows:** Distribution of individual SQL statement execution times.
- **When to act:** If the distribution shifts toward the 500ms–1s range, database performance has degraded. Check DB server CPU/IO load, index health, and lock contention.

---

## Row 5 — Message Broker Statuses

> **When to use:** Investigating failures in message publishing or consuming, or verifying broker registration.

### Kafka Instances

- **Metric:** `maas_health_broker_status{broker_type="Kafka", broker_id="$kafka_instances"}`
- One stat tile is shown per registered Kafka instance.

| Value | Color | Meaning |
|---|---|---|
| `0` | Blue | UNKNOWN — health check hasn't completed yet |
| `10` | Green | OK — broker is reachable |
| `20` | Orange | WARNING |
| `30` | Dark-red | PROBLEM — broker is unreachable |

### Rabbit Instance

- **Metric:** `maas_health_broker_status{broker_type="RabbitMQ", broker_id="$rabbit_instances"}`
- Same value/color mapping as Kafka above.

> If a broker shows **PROBLEM**, MaaS cannot reach it. The service will still accept API requests but any operation requiring that broker (topic creation, vhost provisioning, etc.) will return errors to the caller.

---

## OOB Alerts

**No out-of-the-box alerts are shipped by the MaaS product team.** The product ships metrics and a dashboard only.

If alerting is required, it must be configured by the platform/operations team. 

### Recommended Alerts to Configure

| Alert Name | PromQL Expression | Suggested `for:` | Severity |
|---|---|---|---|
| MaaS Pod Down | `kube_pod_container_status_running{container="maas-service"} == 0` | immediate | critical |
| Master DB Unavailable | `maas_health_is_master_db_alive == 0` | 1m | critical |
| Broker Unreachable | `maas_health_broker_status < 10` | 2m | critical |
| High 5xx Error Rate | `rate(http_requests_total{status_code=~"5.."}[5m]) > 0.1` | 5m | warning |
| DB Connection Pool Saturation | `go_sql_stats_connections_in_use / go_sql_stats_connections_max_open > 0.9` | 5m | warning |
| High Request Latency (P99) | `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{status_code!~"5.."}[5m])) > 5` | 5m | warning |
