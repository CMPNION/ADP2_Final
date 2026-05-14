<!--
Purpose: live demo script for PRR / SRE presentation.
Use this flow to show observability, autoscaling, alerting, and rollout behavior.
-->

# Demo script

## 1. Open the dashboards

- Show the main Grafana dashboard.
- Point out request rate, latency, and error-rate panels.
- Mention the source metrics:
  - `api_gateway_http_requests_total`
  - `api_gateway_http_request_duration_seconds`
  - `order_http_requests_total`
  - `order_http_request_duration_seconds`

## 2. Start the traffic spike

- Run `scripts/traffic_spike.sh`.
- Explain that the script ramps users in stages.
- Keep the dashboard visible while the load increases.

## 3. Show autoscaling

- Open the HPA view in the cluster.
- Show that `api-gateway`, `inventory-service`, and `order-service` scale on CPU and memory.
- Point out the cooldown / stabilization behavior.

## 4. Trigger an alert

- Use the spike to push latency or error rate above the threshold.
- Show the alert firing in Alertmanager or Grafana.
- Explain the error-budget impact in one sentence.

## 5. Demonstrate rollout safety

- Apply a new Deployment version.
- Show that readiness checks keep bad pods out of service.
- Confirm the rollout completes without interrupting traffic.

## 6. Close with the business flow

- `AUTH` → login
- `CATALOG` → create or search products
- `ORDER_CREATE` → create an empty order
- `RESERVE` / `RELEASE` / `CONFIRM` → move stock through the warehouse flow
- `SAFETY_LEVEL` → show low-stock detection

## Presenter note

- Keep the sequence short and visual.
- Show one dashboard, one spike, one HPA reaction, and one alert.
