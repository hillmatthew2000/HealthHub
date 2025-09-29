# Kubernetes namespace
resource "kubernetes_namespace" "healthcare_api" {
  metadata {
    name = "healthcare-api"
    labels = {
      name        = "healthcare-api"
      environment = var.environment
    }
  }

  depends_on = [module.eks]
}

# Kubernetes Secret for database credentials
resource "kubernetes_secret" "db_credentials" {
  metadata {
    name      = "healthcare-api-db-secret"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
  }

  data = {
    DB_URL      = "postgresql://${aws_db_instance.postgres.username}:${random_password.db_password.result}@${aws_db_instance.postgres.endpoint}:${aws_db_instance.postgres.port}/${aws_db_instance.postgres.db_name}?sslmode=require"
    DB_HOST     = aws_db_instance.postgres.endpoint
    DB_PORT     = tostring(aws_db_instance.postgres.port)
    DB_NAME     = aws_db_instance.postgres.db_name
    DB_USER     = aws_db_instance.postgres.username
    DB_PASSWORD = random_password.db_password.result
  }

  type = "Opaque"
}

# Kubernetes Secret for application secrets
resource "kubernetes_secret" "app_secrets" {
  metadata {
    name      = "healthcare-api-app-secret"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
  }

  data = {
    JWT_SECRET     = random_password.jwt_secret.result
    ENCRYPTION_KEY = random_password.encryption_key.result
    REDIS_URL      = "redis://${aws_elasticache_cluster.redis.cache_nodes[0].address}:${aws_elasticache_cluster.redis.cache_nodes[0].port}"
  }

  type = "Opaque"
}

# Kubernetes ConfigMap
resource "kubernetes_config_map" "app_config" {
  metadata {
    name      = "healthcare-api-config"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
  }

  data = {
    ENVIRONMENT = var.environment
    LOG_LEVEL   = var.environment == "prod" ? "info" : "debug"
    GIN_MODE    = var.environment == "prod" ? "release" : "debug"
    PORT        = "8080"
    AWS_REGION  = var.aws_region
  }
}

# Kubernetes Deployment
resource "kubernetes_deployment" "healthcare_api" {
  metadata {
    name      = "healthcare-api"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
    labels = {
      app         = "healthcare-api"
      environment = var.environment
    }
  }

  spec {
    replicas = var.app_replicas

    selector {
      match_labels = {
        app = "healthcare-api"
      }
    }

    template {
      metadata {
        labels = {
          app         = "healthcare-api"
          environment = var.environment
        }
        annotations = {
          "prometheus.io/scrape" = "true"
          "prometheus.io/port"   = "8080"
          "prometheus.io/path"   = "/metrics"
        }
      }

      spec {
        security_context {
          run_as_user     = 65534
          run_as_group    = 65534
          run_as_non_root = true
          fs_group        = 65534
        }

        container {
          name  = "healthcare-api"
          image = "${aws_ecr_repository.app.repository_url}:${var.app_image_tag}"

          port {
            container_port = 8080
            name          = "http"
          }

          env_from {
            config_map_ref {
              name = kubernetes_config_map.app_config.metadata[0].name
            }
          }

          env_from {
            secret_ref {
              name = kubernetes_secret.db_credentials.metadata[0].name
            }
          }

          env_from {
            secret_ref {
              name = kubernetes_secret.app_secrets.metadata[0].name
            }
          }

          resources {
            requests = {
              cpu    = var.app_cpu_request
              memory = var.app_memory_request
            }
            limits = {
              cpu    = var.app_cpu_limit
              memory = var.app_memory_limit
            }
          }

          liveness_probe {
            http_get {
              path = "/health/live"
              port = 8080
            }
            initial_delay_seconds = 30
            period_seconds        = 10
            timeout_seconds       = 5
            failure_threshold     = 3
          }

          readiness_probe {
            http_get {
              path = "/health/ready"
              port = 8080
            }
            initial_delay_seconds = 5
            period_seconds        = 5
            timeout_seconds       = 3
            failure_threshold     = 3
          }

          security_context {
            run_as_user                = 65534
            run_as_group               = 65534
            run_as_non_root            = true
            read_only_root_filesystem  = true
            allow_privilege_escalation = false
            capabilities {
              drop = ["ALL"]
            }
          }

          volume_mount {
            name       = "tmp"
            mount_path = "/tmp"
          }
        }

        volume {
          name = "tmp"
          empty_dir {}
        }

        restart_policy = "Always"
      }
    }
  }

  depends_on = [
    kubernetes_secret.db_credentials,
    kubernetes_secret.app_secrets,
    kubernetes_config_map.app_config
  ]
}

# Kubernetes Service
resource "kubernetes_service" "healthcare_api" {
  metadata {
    name      = "healthcare-api-service"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
    labels = {
      app = "healthcare-api"
    }
    annotations = {
      "service.beta.kubernetes.io/aws-load-balancer-type"                              = "nlb"
      "service.beta.kubernetes.io/aws-load-balancer-backend-protocol"                  = "http"
      "service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled" = "true"
      "service.beta.kubernetes.io/aws-load-balancer-scheme"                            = "internet-facing"
    }
  }

  spec {
    selector = {
      app = "healthcare-api"
    }

    port {
      name        = "http"
      port        = 80
      target_port = 8080
      protocol    = "TCP"
    }

    type = "LoadBalancer"
  }

  depends_on = [kubernetes_deployment.healthcare_api]
}

# Kubernetes Horizontal Pod Autoscaler
resource "kubernetes_horizontal_pod_autoscaler_v2" "healthcare_api" {
  metadata {
    name      = "healthcare-api-hpa"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
  }

  spec {
    scale_target_ref {
      api_version = "apps/v1"
      kind        = "Deployment"
      name        = kubernetes_deployment.healthcare_api.metadata[0].name
    }

    min_replicas = var.node_min_capacity
    max_replicas = var.node_max_capacity

    metric {
      type = "Resource"
      resource {
        name = "cpu"
        target {
          type                = "Utilization"
          average_utilization = 70
        }
      }
    }

    metric {
      type = "Resource"
      resource {
        name = "memory"
        target {
          type                = "Utilization"
          average_utilization = 80
        }
      }
    }

    behavior {
      scale_down {
        stabilization_window_seconds = 300
        policy {
          type          = "Percent"
          value         = 10
          period_seconds = 60
        }
      }

      scale_up {
        stabilization_window_seconds = 60
        policy {
          type          = "Percent"
          value         = 100
          period_seconds = 15
        }
      }
    }
  }

  depends_on = [kubernetes_deployment.healthcare_api]
}

# Data source to get the service information after creation
data "kubernetes_service" "app_service" {
  metadata {
    name      = kubernetes_service.healthcare_api.metadata[0].name
    namespace = kubernetes_service.healthcare_api.metadata[0].namespace
  }

  depends_on = [kubernetes_service.healthcare_api]
}