<!--
Purpose: SRE operating guide for PRR and demo sessions.
This file defines the production-readiness contract for the platform.
-->

# SRE guide

## Goal

Keep the platform measurable, scalable, and safe to operate in a demo or review.

## SLIs

| SLI | What it measures | Source |
| --- | --- | --- |
| Availability | Successful responses over total requests | Gateway / service HTTP metrics |
| Latency | p95 request duration | `*_http_request_duration_seconds` histograms |
| Error rate | 5xx / failed RPC ratio | Gateway + service counters |
| Reservation success | Successful reserve/release/confirm flow ratio | Inventory request counters |
| Readiness | Healthy pods responding to readiness probes | Kubernetes probes |

## SLOs

| SLO | Target |
| --- | --- |
| Gateway availability | 99.9% monthly |
| Inventory reservation success | 99.5% monthly |
| p95 latency for core flows | < 500ms |
| Error rate for core flows | < 1% |

## Error budgets

- Monthly error budget = `100% - SLO`.
- If the budget is spent faster than expected:
  - freeze non-critical releases;
  - investigate the failing service first;
  - prefer rollback over forward-fix during live demo time.

## Key metrics already exposed

- `api_gateway_http_requests_total`
- `api_gateway_http_request_duration_seconds`
- `order_http_requests_total`
- `order_http_request_duration_seconds`
- inventory and catalog request counters/histograms follow the same pattern

## Local monitoring stack

- Prometheus: `http://localhost:9091`
- Grafana: `http://localhost:3001`
- Alertmanager: `http://localhost:9093`
- Gateway metrics listener: `METRICS_ADDR=:9095`

## HPA target signals

- CPU utilization as the primary scaling signal.
- Memory utilization as the secondary signal.
- Scale up during sustained request bursts.
- Scale down only after traffic has stayed low for a grace period.

## Load-test plan

1. Warm up the system with normal traffic.
2. Increase reserve/release traffic gradually.
3. Trigger a short spike to validate autoscaling.
4. Keep the spike long enough to observe HPA reaction.

## Demo checklist

- dashboard shows request rate and latency;
- HPA scales at least one workload;
- alerts can be triggered from a spike;
- rollout and recovery are visible in the cluster.
