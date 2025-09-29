# Healthcare API AWS Deployment Guide

This guide walks you through deploying the Healthcare API to AWS using Terraform and Kubernetes.

## Prerequisites

### Tools Required
- [AWS CLI v2](https://aws.amazon.com/cli/)
- [Terraform](https://www.terraform.io/downloads.html) (>= 1.0)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Docker](https://docs.docker.com/get-docker/)
- [Go](https://golang.org/dl/) (>= 1.21)

### AWS Setup
1. **AWS Account**: Active AWS account with appropriate permissions
2. **IAM User**: Create IAM user with the following policies:
   - `AmazonEKSClusterPolicy`
   - `AmazonEKSWorkerNodePolicy`
   - `AmazonEKS_CNI_Policy`
   - `AmazonEC2ContainerRegistryFullAccess`
   - `AmazonRDSFullAccess`
   - `AmazonElastiCacheFullAccess`
   - `IAMFullAccess`
   - `AmazonVPCFullAccess`
   - `CloudWatchFullAccess`
   - `AWSBackupFullAccess`

3. **S3 Bucket for Terraform State**: Create a bucket for storing Terraform state
4. **DynamoDB Table for State Locking**: Create table with primary key `LockID` (String)

## Quick Start

### 1. Clone and Prepare
```bash
git clone <your-repo>
cd HealthHub
```

### 2. Configure AWS Credentials
```bash
aws configure
# Enter your AWS Access Key ID, Secret Access Key, and region
```

### 3. Prepare Terraform Configuration
```bash
cd deployments/terraform/aws

# Copy example variables
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars with your specific values
# Update main.tf backend configuration with your S3 bucket
```

### 4. Initialize and Deploy Infrastructure
```bash
# Initialize Terraform
terraform init

# Review the deployment plan
terraform plan

# Apply the infrastructure
terraform apply
```

### 5. Configure kubectl
```bash
# Update kubeconfig to connect to the new EKS cluster
aws eks update-kubeconfig --region us-west-2 --name healthhub-dev-eks
```

### 6. Build and Push Application Image
```bash
# Build the Docker image
docker build -t healthcare-api .

# Tag and push to ECR (replace with your ECR URL from Terraform output)
ECR_URL=$(terraform output -raw ecr_repository_url)
docker tag healthcare-api:latest $ECR_URL:latest
docker push $ECR_URL:latest
```

### 7. Deploy Application to Kubernetes
```bash
# Apply Kubernetes manifests
kubectl apply -f ../k8s/

# Check deployment status
kubectl get pods -n healthcare-api
kubectl get services -n healthcare-api
```

## Detailed Configuration

### Terraform Variables

Edit `terraform.tfvars` to customize your deployment:

```hcl
# Environment configuration
environment = "dev"
aws_region  = "us-west-2"
owner       = "your-team"

# Networking
vpc_cidr = "10.0.0.0/16"
availability_zones = ["us-west-2a", "us-west-2b", "us-west-2c"]

# EKS Configuration
kubernetes_version = "1.28"
node_instance_types = ["t3.medium"]
node_desired_capacity = 3

# Database Configuration
db_instance_class = "db.t3.small"
db_allocated_storage = 50

# Security Features (enable for production)
enable_waf = true
enable_cloudtrail = true
enable_guardduty = true
enable_backups = true
```

### Environment-Specific Configurations

#### Development Environment
```bash
terraform apply -var="environment=dev" -var="enable_backups=false" -var="db_deletion_protection=false"
```

#### Production Environment
```bash
terraform apply -var="environment=prod" -var="enable_waf=true" -var="enable_cloudtrail=true" -var="enable_guardduty=true"
```

## Security Configuration

### 1. Database Security
- Encrypted at rest using AWS KMS
- VPC isolation with private subnets
- Security groups restricting access
- Enhanced monitoring enabled

### 2. Application Security
- Containers run as non-root user
- Read-only filesystem
- Security contexts applied
- Network policies (when available)

### 3. Network Security
- VPC with private/public subnet separation
- NAT Gateway for outbound internet access
- Security groups with least privilege
- Optional WAF for web application protection

### 4. Secrets Management
- AWS Secrets Manager for sensitive data
- KMS encryption for all secrets
- Kubernetes secrets for application configuration

## Monitoring and Observability

### CloudWatch Integration
- Application logs centralized in CloudWatch
- Custom metrics and alarms
- Dashboard for key metrics

### Prometheus Integration (Optional)
```bash
# Enable Prometheus in terraform.tfvars
enable_prometheus = true

# Deploy Prometheus operator
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
```

### Health Checks
The application provides several health check endpoints:
- `/health`: Basic health check
- `/health/ready`: Readiness probe (includes DB connectivity)
- `/health/live`: Liveness probe
- `/metrics`: Prometheus metrics

## Backup and Recovery

### Automated Backups
When `enable_backups = true`:
- RDS automated backups (configurable retention)
- AWS Backup service for comprehensive backup
- Cross-region backup replication (optional)

### Manual Backup
```bash
# Create manual RDS snapshot
aws rds create-db-snapshot \
  --db-instance-identifier healthhub-dev-postgres \
  --db-snapshot-identifier healthhub-manual-$(date +%Y%m%d)
```

## Scaling

### Horizontal Pod Autoscaling
HPA is configured automatically based on CPU and memory:
```bash
# Check HPA status
kubectl get hpa -n healthcare-api

# Manually scale deployment
kubectl scale deployment healthcare-api --replicas=5 -n healthcare-api
```

### Cluster Autoscaling
EKS managed node groups automatically scale based on demand:
- Min: 1 node
- Max: 10 nodes
- Desired: 3 nodes

## Troubleshooting

### Common Issues

#### 1. Pod Startup Issues
```bash
# Check pod status and logs
kubectl describe pod <pod-name> -n healthcare-api
kubectl logs <pod-name> -n healthcare-api

# Check events
kubectl get events -n healthcare-api --sort-by='.lastTimestamp'
```

#### 2. Database Connection Issues
```bash
# Verify secrets are properly created
kubectl get secrets -n healthcare-api
kubectl describe secret healthcare-api-db-secret -n healthcare-api

# Test database connectivity from within cluster
kubectl run -it --rm debug --image=postgres:15 --restart=Never -- bash
psql $DB_URL
```

#### 3. Load Balancer Issues
```bash
# Check service status
kubectl describe service healthcare-api-service -n healthcare-api

# Verify security groups allow traffic
aws ec2 describe-security-groups --group-ids <sg-id>
```

#### 4. Certificate Issues (HTTPS)
```bash
# Check certificate status
kubectl describe ingress healthcare-api-ingress -n healthcare-api

# Verify cert-manager is working
kubectl get certificates -n healthcare-api
kubectl describe certificate healthcare-api-tls -n healthcare-api
```

### Performance Tuning

#### Database Optimization
```sql
-- Connect to database and run performance queries
SELECT * FROM pg_stat_activity;
SELECT schemaname,tablename,attname,avg_width,n_distinct FROM pg_stats;
```

#### Application Performance
```bash
# Check resource usage
kubectl top pods -n healthcare-api
kubectl top nodes

# Review metrics
curl http://<load-balancer-url>/metrics
```

## CI/CD Integration

### GitHub Actions Setup

1. **Repository Secrets**:
   ```
   AWS_ACCESS_KEY_ID
   AWS_SECRET_ACCESS_KEY
   TF_STATE_BUCKET
   TF_LOCK_TABLE
   CODECOV_TOKEN (optional)
   SLACK_WEBHOOK (optional)
   ```

2. **Workflow Triggers**:
   - Push to `main`: Deploys to production
   - Push to `develop`: Deploys to development
   - Pull requests: Runs tests and security scans

3. **Manual Deployment**:
   ```bash
   # Trigger manual deployment via GitHub Actions
   gh workflow run deploy.yml -f environment=prod -f action=apply
   ```

## Cost Optimization

### Development Environment
```hcl
# Minimal cost configuration
db_instance_class = "db.t3.micro"
node_instance_types = ["t3.small"]
enable_backups = false
enable_guardduty = false
enable_cloudtrail = false
```

### Production Environment
```hcl
# Production configuration with cost optimization
db_instance_class = "db.t3.medium"
node_instance_types = ["t3.medium", "t3.large"]
enable_backups = true
redis_node_type = "cache.t3.small"
```

## Compliance and Auditing

### HIPAA Compliance Considerations
1. **Encryption**: All data encrypted in transit and at rest
2. **Access Logging**: CloudTrail enabled for audit trails
3. **Network Isolation**: VPC with private subnets
4. **User Access**: RBAC configured for least privilege
5. **Backup**: Automated backups with encryption

### Audit Logging
```bash
# View CloudTrail logs
aws logs describe-log-groups --log-group-name-prefix CloudTrail

# Application audit logs
kubectl logs deployment/healthcare-api -n healthcare-api | grep -i audit
```

## Cleanup

### Destroy Infrastructure
```bash
# Destroy Terraform-managed resources
cd deployments/terraform/aws
terraform destroy

# Note: Some resources may need manual cleanup:
# - ECR images
# - CloudWatch logs (if retention is set)
# - EBS snapshots
```

### Cost Monitoring
```bash
# Check current costs
aws ce get-cost-and-usage \
  --time-period Start=2024-01-01,End=2024-01-31 \
  --granularity MONTHLY \
  --metrics BlendedCost
```

## Support and Maintenance

### Regular Maintenance Tasks
1. **Security Updates**: Regularly update base images and dependencies
2. **Certificate Renewal**: Monitor SSL certificate expiration
3. **Backup Testing**: Periodically test backup restoration
4. **Performance Review**: Monitor application metrics and optimize
5. **Cost Review**: Regular cost analysis and optimization

### Getting Help
- Check application logs: `kubectl logs -f deployment/healthcare-api -n healthcare-api`
- Review AWS CloudWatch: Monitor dashboards and alarms
- Infrastructure issues: Check Terraform state and AWS console
- Application issues: Review Go application logs and metrics