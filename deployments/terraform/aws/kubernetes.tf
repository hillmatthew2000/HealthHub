# Terraform provisions what the application needs from the infrastructure:
# its namespace and the Secret holding the database URL and the generated
# keys. The workload itself (Deployment, Service, Ingress, autoscaler and
# configuration) is defined once, in deployments/k8s, and applied by the
# deploy workflow, so the two never fight over the same objects.

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

# The keys and names match the secretRef in deployments/k8s/deployment.yaml.
resource "kubernetes_secret" "app" {
  metadata {
    name      = "healthcare-api-secrets"
    namespace = kubernetes_namespace.healthcare_api.metadata[0].name
    labels = {
      app = "healthcare-api"
    }
  }

  data = {
    DATABASE_URL   = "postgres://${aws_db_instance.postgres.username}:${urlencode(random_password.db_password.result)}@${aws_db_instance.postgres.address}:${aws_db_instance.postgres.port}/${aws_db_instance.postgres.db_name}?sslmode=require"
    JWT_SECRET     = random_password.jwt_secret.result
    ENCRYPTION_KEY = random_password.encryption_key.result
  }

  type = "Opaque"
}

# Earlier versions managed the workload here as well. Forget those objects
# without deleting them; the kustomization in deployments/k8s adopts them
# under the same names on its next apply.
removed {
  from = kubernetes_deployment.healthcare_api
  lifecycle {
    destroy = false
  }
}

removed {
  from = kubernetes_service.healthcare_api
  lifecycle {
    destroy = false
  }
}

removed {
  from = kubernetes_horizontal_pod_autoscaler_v2.healthcare_api
  lifecycle {
    destroy = false
  }
}

removed {
  from = kubernetes_config_map.app_config
  lifecycle {
    destroy = false
  }
}
