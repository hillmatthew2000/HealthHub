# Kubernetes deployment

Manifests for running the HealthHub API on Kubernetes, applied with
kustomize. They define the workload only: a Deployment, Service, Ingress,
HorizontalPodAutoscaler, PodDisruptionBudget and ConfigMap.

The namespace and the `healthcare-api-secrets` Secret are not part of the
kustomization, so applying it can never replace real credentials. On the
AWS environment Terraform creates both
(`deployments/terraform/aws/kubernetes.tf`).

## Prerequisites

- Kubernetes 1.27 or later
- the NGINX ingress controller and cert-manager
- a PostgreSQL database reachable from the cluster

## First deployment on a cluster not managed by Terraform

```bash
kubectl create namespace healthcare-api

kubectl create secret generic healthcare-api-secrets -n healthcare-api \
  --from-literal=DATABASE_URL='postgres://USER:PASSWORD@HOST:5432/healthhub?sslmode=require' \
  --from-literal=JWT_SECRET="$(openssl rand -base64 48)" \
  --from-literal=ENCRYPTION_KEY="$(openssl rand -base64 32)"
```

`ENCRYPTION_KEY` encrypts patient data at rest. Back it up somewhere other
than the cluster: data written with a lost key cannot be recovered.

## Deploying

```bash
cd deployments/k8s
kustomize edit set image healthcare-api=REGISTRY/healthhub-prod@sha256:DIGEST
kubectl apply -k .
kubectl rollout status deployment/healthcare-api -n healthcare-api
```

The deploy workflow (`.github/workflows/deploy.yml`) does exactly this with
the digest of the image it has just built and scanned. Pending database
migrations are applied by the first pod to start; concurrent pods wait on a
database lock.

### Creating the first administrator

Self-registration is off by default. Bootstrap an administrator from a
one-off pod that uses the same image, configuration and Secret:

```bash
kubectl run healthhub-create-admin -n healthcare-api --rm -i --restart=Never \
  --image=REGISTRY/healthhub-prod@sha256:DIGEST \
  --overrides='{"spec":{"containers":[{"name":"healthhub-create-admin","image":"REGISTRY/healthhub-prod@sha256:DIGEST","stdin":true,"args":["create-admin","-email","admin@example.org","-first-name","Ada","-last-name","Admin"],"envFrom":[{"configMapRef":{"name":"healthcare-api-config"}},{"secretRef":{"name":"healthcare-api-secrets"}}]}]}}' \
  <<< 'the initial admin password'
```

Further accounts are created through `POST /api/v1/users`.

## Configuration

`configmap.yaml` holds the non-secret settings. Review at least:

- `ALLOWED_ORIGINS`: the browser origins allowed to call the API.
- `TRUSTED_PROXIES`: the addresses of the ingress controller pods. The API
  uses them to find the real client address for rate limiting and audit
  records.
- `RATE_LIMIT_RPM` and `AUTH_RATE_LIMIT_RPM`: per-client limits, enforced
  by each pod separately.

`ingress.yaml` names the public host (`api.yourdomain.com`) and the
cert-manager issuer.

## Health checks

- `/health/live`: liveness and startup probes. The process is serving.
- `/health/ready`: readiness probe. The database is reachable.

## Troubleshooting

```bash
kubectl get pods -n healthcare-api
kubectl describe pod POD -n healthcare-api
kubectl logs deployment/healthcare-api -n healthcare-api
kubectl get events -n healthcare-api --sort-by=.lastTimestamp
```

The image has no shell, so `kubectl exec` cannot be used to look around a
running container. Check its configuration with
`kubectl get configmap healthcare-api-config -n healthcare-api -o yaml`
and the Secret's keys with
`kubectl describe secret healthcare-api-secrets -n healthcare-api`.

## Security

- Containers run as a non-root user with a read-only root filesystem, no
  privilege escalation, all capabilities dropped and the runtime's default
  seccomp profile.
- The service account token is not mounted.
- Consider an operator such as External Secrets to source the Secret from
  AWS Secrets Manager (Terraform already stores the values there), and
  NetworkPolicies limiting ingress to the ingress controller.
