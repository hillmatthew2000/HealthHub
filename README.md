# Healthcare API - Complete Project

🏥 **A comprehensive, production-ready healthcare API built with Go, implementing FHIR R4 standards with enterprise-grade security, monitoring, and cloud deployment capabilities.**

## 🚀 Features

- **FHIR R4 Compliance**: Full implementation of Patient and Observation resources
- **Enterprise Security**: JWT authentication, RBAC, AES-256 encryption, audit logging
- **Cloud Native**: Docker containers, Kubernetes deployment, AWS infrastructure
- **Monitoring**: Prometheus metrics, Grafana dashboards, health checks
- **API Documentation**: Complete OpenAPI 3.0 specification
- **Production Ready**: Comprehensive testing, CI/CD, backup strategies

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Development Setup](#development-setup)
- [Deployment](#deployment)
- [API Documentation](#api-documentation)
- [Security](#security)
- [Monitoring](#monitoring)
- [Contributing](#contributing)
- [License](#license)

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- PostgreSQL (for local development)
- kubectl (for Kubernetes deployment)

### Local Development

1. **Clone and setup**:
```bash
git clone <repository-url>
cd HealthHub
go mod download
```

2. **Start dependencies**:
```bash
docker-compose up -d postgres redis
```

3. **Set environment variables**:
```bash
export DATABASE_URL="postgres://user:password@localhost:5432/healthcare_db?sslmode=disable"
export JWT_SECRET="your-super-secure-jwt-secret-key-here"
export ENCRYPTION_KEY="your-32-byte-encryption-key-here"
export REDIS_URL="redis://localhost:6379"
export SERVER_PORT="8080"
export LOG_LEVEL="info"
```

4. **Run the application**:
```bash
go run cmd/server/main.go
```

The API will be available at `http://localhost:8080`

### Docker Deployment

```bash
# Build and run with Docker Compose
docker-compose up --build

# Or build and run individual container
docker build -t healthcare-api .
docker run -p 8080:8080 healthcare-api
```

### Health Check

```bash
curl http://localhost:8080/api/v1/health
```

## 📁 Project Structure

```
HealthHub/
├── cmd/server/                 # Application entry point
│   └── main.go                # Main server file
├── internal/                  # Private application code
│   ├── auth/                  # Authentication & authorization
│   ├── config/                # Configuration management
│   ├── handlers/              # HTTP request handlers
│   └── models/                # FHIR data models
├── pkg/                       # Public packages
│   ├── database/              # Database connection & migrations
│   ├── encryption/            # Encryption utilities
│   ├── logger/                # Structured logging
│   └── metrics/               # Prometheus metrics
├── deployments/               # Deployment configurations
│   ├── docker/                # Docker files
│   ├── kubernetes/            # K8s manifests
│   ├── terraform/             # Infrastructure as Code
│   └── monitoring/            # Monitoring stack
├── docs/                      # Documentation
│   ├── openapi.yaml          # API specification
│   └── README.md             # Comprehensive documentation
├── docker-compose.yml        # Local development environment
├── Dockerfile                # Container image definition
├── go.mod                    # Go module definition
└── README.md                 # This file
```

## 🛠 Development Setup

### Environment Configuration

Create a `.env` file for local development:

```env
# Database
DATABASE_URL=postgres://user:password@localhost:5432/healthcare_db?sslmode=disable

# Security
JWT_SECRET=your-super-secure-jwt-secret-key-here-at-least-32-characters
ENCRYPTION_KEY=your-32-byte-encryption-key-here!!

# Redis
REDIS_URL=redis://localhost:6379

# Server
SERVER_PORT=8080
LOG_LEVEL=info
```

### Database Setup

1. **Start PostgreSQL**:
```bash
docker run -d \
  --name postgres \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=healthcare_db \
  -p 5432:5432 \
  postgres:15
```

2. **Run migrations** (handled automatically on startup)

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Security scan
gosec ./...
```

## 🚀 Deployment

### Docker Deployment

#### Building the Image

```bash
# Build production image
docker build -t healthcare-api:latest .

# Build with specific tag
docker build -t healthcare-api:v1.0.0 .
```

#### Running with Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f healthcare-api

# Stop services
docker-compose down
```

### Kubernetes Deployment

#### Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Docker images pushed to registry

#### Deploy to Kubernetes

1. **Apply configurations**:
```bash
# Deploy in order
kubectl apply -f deployments/kubernetes/namespace.yaml
kubectl apply -f deployments/kubernetes/configmap.yaml
kubectl apply -f deployments/kubernetes/secret.yaml
kubectl apply -f deployments/kubernetes/postgres.yaml
kubectl apply -f deployments/kubernetes/redis.yaml
kubectl apply -f deployments/kubernetes/healthcare-api.yaml
kubectl apply -f deployments/kubernetes/service.yaml
kubectl apply -f deployments/kubernetes/ingress.yaml
```

2. **Verify deployment**:
```bash
kubectl get pods -n healthcare-api
kubectl get services -n healthcare-api
kubectl logs -f deployment/healthcare-api -n healthcare-api
```

3. **Access the API**:
```bash
kubectl port-forward svc/healthcare-api 8080:80 -n healthcare-api
curl http://localhost:8080/api/v1/health
```

### Cloud Deployment (AWS)

#### Terraform Infrastructure

1. **Prerequisites**:
```bash
# Install Terraform
# Configure AWS CLI
aws configure
```

2. **Deploy infrastructure**:
```bash
cd deployments/terraform/aws
terraform init
terraform plan
terraform apply
```

3. **Deploy to EKS**:
```bash
# Update kubeconfig
aws eks update-kubeconfig --region us-west-2 --name healthcare-api-cluster

# Deploy application
kubectl apply -f ../../kubernetes/
```

#### Infrastructure Components

- **EKS Cluster**: Managed Kubernetes cluster
- **RDS PostgreSQL**: Managed database with encryption
- **ElastiCache Redis**: Managed Redis cluster
- **ALB**: Application Load Balancer with SSL termination
- **VPC**: Isolated network with public/private subnets
- **Security Groups**: Restrictive network access rules
- **IAM Roles**: Least privilege access control
- **CloudWatch**: Logging and monitoring
- **Backup**: Automated database backups

## 📖 API Documentation

### Interactive Documentation

- **Swagger UI**: Available at `/docs` when running locally
- **OpenAPI Spec**: Complete specification in `docs/openapi.yaml`
- **Comprehensive Guide**: Detailed documentation in `docs/README.md`

### Key Endpoints

#### Authentication
```bash
POST /api/v1/auth/register    # Register new user
POST /api/v1/auth/login       # User login
POST /api/v1/auth/refresh     # Refresh token
POST /api/v1/auth/logout      # User logout
```

#### Patients
```bash
GET    /api/v1/patients       # List patients
POST   /api/v1/patients       # Create patient
GET    /api/v1/patients/{id}  # Get patient
PUT    /api/v1/patients/{id}  # Update patient
DELETE /api/v1/patients/{id}  # Delete patient
```

#### Observations
```bash
GET    /api/v1/observations       # List observations
POST   /api/v1/observations       # Create observation
GET    /api/v1/observations/{id}  # Get observation
PUT    /api/v1/observations/{id}  # Update observation
DELETE /api/v1/observations/{id}  # Delete observation
```

#### Health Checks
```bash
GET /api/v1/health        # Basic health check
GET /api/v1/health/ready  # Readiness probe
GET /api/v1/health/live   # Liveness probe
GET /metrics              # Prometheus metrics
```

### Example API Usage

#### Create a Patient
```bash
curl -X POST http://localhost:8080/api/v1/patients \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "active": true,
    "identifier": [{"use": "usual", "value": "MRN12345"}],
    "name": [{"use": "official", "family": "Doe", "given": ["John"]}],
    "gender": "male",
    "birth_date": "1990-05-15"
  }'
```

#### Create an Observation
```bash
curl -X POST http://localhost:8080/api/v1/observations \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "final",
    "code": {
      "coding": [{"system": "http://loinc.org", "code": "8867-4", "display": "Heart rate"}]
    },
    "subject": {"reference": "Patient/PATIENT_ID"},
    "value_quantity": {"value": 72, "unit": "beats/min"}
  }'
```

## 🔒 Security

### Security Features

- **Authentication**: JWT tokens with secure random generation
- **Authorization**: Role-based access control (RBAC)
- **Encryption**: 
  - TLS 1.3 for data in transit
  - AES-256 for data at rest
  - Application-level encryption for sensitive fields
- **Input Validation**: Comprehensive request validation
- **Rate Limiting**: Configurable rate limits per user role
- **Audit Logging**: Complete audit trail of all operations
- **Security Headers**: OWASP recommended security headers

### RBAC Roles

- **admin**: Full system access, user management
- **doctor**: Read/write access to all patient data
- **nurse**: Read/write access to assigned patients
- **patient**: Read access to own data only

### Compliance

- **HIPAA Ready**: Designed with HIPAA compliance in mind
- **GDPR Considerations**: Data protection and privacy features
- **Audit Trail**: Comprehensive logging for compliance requirements

### Security Configuration

```yaml
# Security settings in config
security:
  jwt:
    secret: "${JWT_SECRET}"
    expiry: 1h
    refresh_expiry: 30d
  encryption:
    key: "${ENCRYPTION_KEY}"
    algorithm: AES-256-GCM
  rate_limiting:
    enabled: true
    requests_per_minute: 100
  audit:
    enabled: true
    log_level: INFO
```

## 📊 Monitoring

### Monitoring Stack

The application includes comprehensive monitoring with:

- **Prometheus**: Metrics collection and alerting
- **Grafana**: Dashboards and visualization
- **Health Checks**: Kubernetes probes and application health
- **Structured Logging**: JSON logs with correlation IDs

### Metrics

#### Application Metrics
- HTTP request duration and count
- Database query performance
- Authentication success/failure rates
- Business logic metrics

#### System Metrics
- CPU and memory usage
- Garbage collection metrics
- Goroutine count
- Database connection pool stats

### Grafana Dashboards

Pre-built dashboards include:
- **Healthcare API Overview**: High-level application metrics
- **Infrastructure Monitoring**: System and resource metrics
- **Security Dashboard**: Authentication and access patterns
- **Performance Analytics**: Response times and throughput

### Alerting

Configured alerts for:
- High error rates (>5% for 5 minutes)
- Slow response times (>2s for 5 minutes)
- Database connectivity issues
- Memory usage >80%
- Failed authentication attempts

### Accessing Monitoring

#### Local Development
```bash
# Prometheus
http://localhost:9090

# Grafana (admin/admin)
http://localhost:3000
```

#### Kubernetes
```bash
# Port forward to access
kubectl port-forward svc/prometheus 9090:9090 -n monitoring
kubectl port-forward svc/grafana 3000:3000 -n monitoring
```

## 🧪 Testing

### Test Categories

#### Unit Tests
```bash
# Run unit tests
go test -short ./...

# With coverage
go test -short -cover ./...
```

#### Integration Tests
```bash
# Requires running database
go test -tags=integration ./...
```

#### Load Testing
```bash
# Using k6 (install k6 first)
k6 run tests/load/basic-load-test.js
```

### Test Configuration

Test environment uses:
- In-memory database for unit tests
- Docker containers for integration tests
- Test fixtures for reproducible data
- Mocking for external dependencies

### Continuous Integration

GitHub Actions workflow includes:
- Unit and integration tests
- Security scanning with gosec
- Code quality checks with golangci-lint
- Docker image building and scanning
- Deployment to staging environment

## 🤝 Contributing

### Development Workflow

1. **Fork the repository**
2. **Create a feature branch**:
   ```bash
   git checkout -b feature/amazing-feature
   ```
3. **Make your changes**
4. **Run tests**:
   ```bash
   go test ./...
   ```
5. **Commit changes**:
   ```bash
   git commit -m "Add amazing feature"
   ```
6. **Push to branch**:
   ```bash
   git push origin feature/amazing-feature
   ```
7. **Open a Pull Request**

### Code Standards

- Follow Go formatting conventions (`go fmt`)
- Write comprehensive tests for new features
- Update documentation for API changes
- Follow semantic versioning for releases
- Ensure security best practices

### Commit Message Format

```
type(scope): subject

body

footer
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Example:
```
feat(auth): add multi-factor authentication

Implement TOTP-based MFA for enhanced security.
Includes backup codes and recovery options.

Closes #123
```

## 🆘 Support

### Documentation

- **API Reference**: `docs/openapi.yaml`
- **Deployment Guide**: `deployments/README.md`
- **Development Setup**: This README

### Getting Help

- **Issues**: GitHub Issues for bug reports and feature requests
- **Discussions**: GitHub Discussions for questions and ideas
- **Email**: support@healthcare-api.com

### Commercial Support

For production deployments and commercial support:
- **Enterprise Support**: Available with SLA guarantees
- **Professional Services**: Implementation and customization services
- **Training**: Developer and operations training programs

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **FHIR Community**: For healthcare interoperability standards
- **Go Community**: For excellent libraries and tools
- **Healthcare Organizations**: For feedback and requirements
- **Security Researchers**: For vulnerability reports and improvements

---

## 📊 Project Status

- **Version**: 1.0.0
- **Status**: Production Ready
- **Last Updated**: January 15, 2024
- **Maintainers**: Healthcare API Team

### Recent Updates

- ✅ Complete FHIR R4 implementation
- ✅ Production-grade security features
- ✅ Kubernetes deployment manifests
- ✅ Comprehensive monitoring setup
- ✅ Complete API documentation
- ✅ CI/CD pipeline implementation

### Roadmap

- **Q2 2024**: Additional FHIR resources (Practitioner, Organization)
- **Q3 2024**: SMART on FHIR integration
- **Q4 2024**: Bulk data export capabilities
- **2025**: GraphQL API support

---

**Built with ❤️ for the healthcare community**
