# Healthcare API Monitoring Setup

This directory contains monitoring configurations for the Healthcare API using Prometheus and Grafana.

## Overview

The monitoring stack includes:
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Visualization and dashboards
- **AlertManager**: Alert routing and management (optional)

## Quick Setup

### 1. Deploy Monitoring Stack

```bash
# Create monitoring namespace and deploy Prometheus
kubectl apply -f prometheus.yaml

# Deploy Grafana with pre-configured dashboards
kubectl apply -f grafana.yaml

# Verify deployments
kubectl get pods -n monitoring
kubectl get services -n monitoring
```

### 2. Access Dashboards

```bash
# Port forward to access Grafana locally
kubectl port-forward service/grafana-service 3000:3000 -n monitoring

# Open browser to http://localhost:3000
# Default credentials: admin/admin123
```

### 3. Access Prometheus

```bash
# Port forward to access Prometheus UI
kubectl port-forward service/prometheus-service 9090:9090 -n monitoring

# Open browser to http://localhost:9090
```

## Metrics Collected

### Application Metrics

#### HTTP Metrics
- `http_requests_total`: Total HTTP requests by method, endpoint, and status code
- `http_request_duration_seconds`: Request duration histogram
- `http_request_size_bytes`: Request size histogram
- `http_response_size_bytes`: Response size histogram

#### Database Metrics
- `database_connections_total`: Total database connections configured
- `database_connections_active`: Currently active database connections
- `database_query_duration_seconds`: Database query duration histogram
- `database_transactions_total`: Total database transactions by operation and status

#### Business Metrics
- `patients_total`: Total number of patients in the system
- `observations_total`: Total number of observations in the system
- `auth_attempts_total`: Authentication attempts by method and status
- `auth_tokens_active`: Number of active authentication tokens

#### System Metrics
- `goroutines_active`: Number of active goroutines
- `memory_usage_bytes`: Current memory usage
- `gc_duration_seconds`: Garbage collection duration

### Kubernetes Metrics

The Prometheus configuration also scrapes:
- Node metrics (CPU, memory, disk, network)
- Pod metrics (resource usage, restarts)
- Container metrics (cAdvisor)
- Kubernetes API server metrics

## Alerts

### Pre-configured Alerts

1. **HealthcareAPIDown**: API service is unreachable
2. **HighErrorRate**: Error rate above 10% for 5 minutes
3. **HighResponseTime**: 95th percentile response time above 500ms
4. **DatabaseConnections**: Connection pool usage above 80%
5. **PodCrashLooping**: Pod restarting repeatedly

### Alert Configuration

Alerts are defined in the Prometheus ConfigMap (`alerts.yml`). To modify alerts:

```bash
# Edit the prometheus configmap
kubectl edit configmap prometheus-config -n monitoring

# Reload Prometheus configuration
kubectl exec -it deployment/prometheus -n monitoring -- killall -HUP prometheus
```

## Dashboards

### Healthcare API Dashboard

The pre-configured dashboard includes:

1. **Request Rate**: Requests per second by endpoint and method
2. **Response Time**: 50th and 95th percentile response times
3. **Error Rate**: Percentage of 5xx responses
4. **Database Connections**: Active vs total database connections
5. **System Resources**: Memory, CPU, and goroutine usage

### Custom Dashboards

To add custom dashboards:

1. Create dashboard in Grafana UI
2. Export JSON configuration
3. Add to the `grafana-config` ConfigMap under `dashboards/`
4. Restart Grafana deployment

## Configuration

### Prometheus Configuration

Key configuration files in the Prometheus ConfigMap:

- `prometheus.yml`: Main Prometheus configuration
- `alerts.yml`: Alerting rules

### Grafana Configuration

Key configuration in the Grafana ConfigMap:

- `grafana.ini`: Grafana server configuration
- `datasources.yaml`: Data source configuration (Prometheus)
- `dashboards.yaml`: Dashboard provider configuration
- `healthcare-api-dashboard.json`: Pre-built dashboard

### Scrape Targets

Prometheus is configured to scrape:

1. **Kubernetes API Server**: Cluster health metrics
2. **Kubernetes Nodes**: Node-level system metrics
3. **cAdvisor**: Container resource usage
4. **Healthcare API**: Application metrics via `/metrics` endpoint
5. **Service Endpoints**: Services with `prometheus.io/scrape: "true"` annotation

## Security Considerations

### Authentication
- Grafana requires username/password (changeable)
- Prometheus has no built-in authentication (use ingress/proxy)

### Network Security
- Services use ClusterIP (internal only)
- Use ingress controllers with TLS for external access
- Consider network policies to restrict access

### RBAC
- Prometheus service account has cluster-wide read permissions
- Required for Kubernetes service discovery
- Review permissions for security compliance

## Production Setup

### External Storage

For production, configure persistent storage:

```yaml
# Add to Prometheus deployment
volumes:
- name: prometheus-storage-volume
  persistentVolumeClaim:
    claimName: prometheus-pvc

# Add to Grafana deployment  
volumes:
- name: grafana-storage
  persistentVolumeClaim:
    claimName: grafana-pvc
```

### High Availability

For HA setup:
1. Run multiple Prometheus replicas with shared storage
2. Use Prometheus federation or Thanos for long-term storage
3. Run multiple Grafana instances behind a load balancer

### AlertManager

Deploy AlertManager for alert routing:

```bash
# Deploy AlertManager (configuration not included)
kubectl apply -f alertmanager.yaml

# Configure in prometheus.yml:
alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

## Troubleshooting

### Common Issues

#### Metrics Not Appearing
```bash
# Check if application exposes metrics
kubectl exec -it <pod-name> -n healthcare-api -- curl localhost:8080/metrics

# Verify Prometheus can reach the target
kubectl logs deployment/prometheus -n monitoring
```

#### Dashboard Not Loading
```bash
# Check Grafana logs
kubectl logs deployment/grafana -n monitoring

# Verify data source connection
kubectl exec -it deployment/grafana -n monitoring -- curl localhost:3000/api/health
```

#### High Memory Usage
```bash
# Check Prometheus storage usage
kubectl exec -it deployment/prometheus -n monitoring -- du -sh /prometheus

# Consider reducing retention time or adding storage limits
```

### Useful Commands

```bash
# Check metrics endpoint
kubectl port-forward deployment/healthcare-api 8080:8080 -n healthcare-api
curl http://localhost:8080/metrics

# Prometheus targets status
kubectl port-forward service/prometheus-service 9090:9090 -n monitoring
# Visit http://localhost:9090/targets

# Grafana dashboard development
kubectl port-forward service/grafana-service 3000:3000 -n monitoring
# Visit http://localhost:3000
```

## Metrics Reference

### HTTP Request Metrics
```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status_code=~"5.."}[5m]) / rate(http_requests_total[5m])

# Response time percentiles
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

### Database Metrics
```promql
# Connection pool utilization
database_connections_active / database_connections_total

# Query duration by operation
rate(database_query_duration_seconds_sum[5m]) / rate(database_query_duration_seconds_count[5m])
```

### Business Metrics
```promql
# Patient growth rate
rate(patients_total[1h])

# Authentication success rate
rate(auth_attempts_total{status="success"}[5m]) / rate(auth_attempts_total[5m])
```

## Integration with CI/CD

### Automated Alerting
Configure alerts to notify on:
- Deployment failures
- Performance regressions
- Error rate increases

### Monitoring as Code
- Store dashboard configurations in Git
- Use Jsonnet or similar tools for dashboard generation
- Automate dashboard deployment in CI/CD pipeline

### SLI/SLO Monitoring
Define and monitor Service Level Indicators:
- Availability (uptime percentage)
- Response time (95th percentile < 500ms)
- Error rate (< 1% over 1 hour)
- Throughput (requests per second)