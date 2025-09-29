# Healthcare API Kubernetes Deployment

This directory contains Kubernetes manifests for deploying the Healthcare API to a Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (v1.20+)
- kubectl configured
- NGINX Ingress Controller
- cert-manager (for SSL certificates)
- PostgreSQL database (can be deployed separately or using cloud provider)

## Quick Deploy

```bash
# Apply all manifests
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -n healthcare-api
kubectl get services -n healthcare-api
kubectl get ingress -n healthcare-api
```

## Configuration

### Secrets

Update the following base64 encoded values in `secret.yaml`:

```bash
# Database URL
echo -n "postgresql://user:password@host:5432/healthhub" | base64

# JWT Secret
echo -n "your-jwt-secret-key" | base64

# Encryption Key (32 bytes)
echo -n "your-32-byte-encryption-key-here" | base64
```

### ConfigMap

Update `configmap.yaml` with your environment-specific settings:

- `DB_HOST`: PostgreSQL host
- `DB_PORT`: PostgreSQL port
- `REDIS_HOST`: Redis host (optional)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)

### Ingress

Update `ingress.yaml` with your domain:

- Replace `api.yourdomain.com` with your actual domain
- Update CORS origins if needed
- Configure SSL certificate issuer

## Monitoring

Check application health:

```bash
# Pod status
kubectl get pods -n healthcare-api

# Pod logs
kubectl logs -f deployment/healthcare-api -n healthcare-api

# Service endpoints
kubectl get endpoints -n healthcare-api

# Ingress status
kubectl describe ingress healthcare-api-ingress -n healthcare-api
```

## Scaling

```bash
# Scale deployment
kubectl scale deployment healthcare-api --replicas=5 -n healthcare-api

# Check horizontal pod autoscaler (if configured)
kubectl get hpa -n healthcare-api
```

## Troubleshooting

### Common Issues

1. **Pod not starting**: Check logs and secrets
```bash
kubectl describe pod <pod-name> -n healthcare-api
kubectl logs <pod-name> -n healthcare-api
```

2. **Database connection issues**: Verify secrets and network policies
```bash
kubectl exec -it <pod-name> -n healthcare-api -- env | grep DB_
```

3. **Ingress not working**: Check ingress controller and DNS
```bash
kubectl describe ingress healthcare-api-ingress -n healthcare-api
kubectl get events -n healthcare-api
```

### Health Checks

The application exposes the following endpoints:

- `/health`: Basic health check
- `/health/ready`: Readiness probe (includes DB connectivity)
- `/health/live`: Liveness probe
- `/metrics`: Prometheus metrics (if enabled)

## Security Considerations

- All containers run as non-root user (UID 65534)
- Read-only root filesystem
- No privilege escalation
- Secrets are base64 encoded (consider using sealed-secrets or external secret operators)
- RBAC is configured for least privilege access
- Network policies should be implemented for production

## Production Recommendations

1. **Use external database**: Don't run PostgreSQL in the same cluster
2. **Implement network policies**: Restrict pod-to-pod communication
3. **Use external secrets management**: HashiCorp Vault, AWS Secrets Manager, etc.
4. **Enable monitoring**: Prometheus, Grafana, AlertManager
5. **Implement backup strategy**: Database and persistent volume backups
6. **Use resource quotas**: Limit resource consumption per namespace
7. **Enable audit logging**: Kubernetes audit logs for compliance