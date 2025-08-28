#!/bin/bash

####################################################################################
# This script manages the installation and uninstallation of the Helm chart
# for the "Registry" service in a Kubernetes cluster. It checks for prerequisites,
# deploys necessary configurations, and ensures that the service is set up correctly.
####################################################################################

set -e

# Configuration
DB_USER="postgres"
DB_PASS="secret"
DB_NAME="registry"
DB_HOST="postgresql"
RABBITMQ_HOST="rabbitmq"
OTEL_HOST="opentelemetry-collector"

# Parameters
COMMAND=$1

####################################################################################

install_registry() {
  if helm ls --namespace default | grep -q '^registry\s'; then
      echo "Registry Helm chart is already installed."
      return 0
  fi

  echo "Creating Registry configuration..."
  mkdir -p /tmp/registry-helm-config
  cp -r local/rabbitmq/certs /tmp/registry-helm-config/

  kubectl create secret generic registry-certs \
      --from-file=ca.crt=/tmp/registry-helm-config/certs/ca.crt \
      --from-file=client.crt=/tmp/registry-helm-config/certs/client.crt \
      --from-file=client.key=/tmp/registry-helm-config/certs/client.key \
      --dry-run=client -o yaml | kubectl apply -f -

  cat > /tmp/registry-helm-config/values.yaml << EOF
  image:
    registry: ""
    repository: registry
    tag: dev

  extraVolumeMounts:
    - name: registry-certs
      mountPath: /etc/registry/certs
      readOnly: true
  extraVolumes:
    - name: registry-certs
      secret:
        secretName: registry-certs
  extraEnvs:
    - name: OTEL_HOST
      value: "${OTEL_HOST}:4317"
  livenessProbe:
    httpGet:
      path: /probe/liveness
      port: http-status
      scheme: HTTP
    failureThreshold: 3
    periodSeconds: 5
  readinessProbe:
    httpGet:
      path: /probe/readiness
      port: http-status
      scheme: HTTP
    failureThreshold: 3
    periodSeconds: 5
  config:
    database:
      host: ${DB_HOST}
      name: ${DB_NAME}
      user:
        value: ${DB_USER}
      password:
        value: ${DB_PASS}
    orbital:
      targets:
        - region: test-region
          connection:
            type: amqp
            amqp:
              url: "amqps://${RABBITMQ_HOST}:5671"
              source: "cmk.global.tenants"
              target: "cmk.emea.tenants"
            auth:
              type: mtls
              mtls:
                certFile: /etc/registry/certs/client.crt
                keyFile: /etc/registry/certs/client.key
                caFile: /etc/registry/certs/ca.crt
EOF

  echo "Deploying Registry Helm chart..."
  helm upgrade --install registry ./charts/registry \
      --values /tmp/registry-helm-config/values.yaml \
      --wait --timeout=60s

  if [ $? -ne 0 ]; then
    echo "ERROR: Helm deployment failed!"
    echo "Checking deployment status..."
    kubectl get pods -l app.kubernetes.io/name=registry -o wide
    kubectl describe pods -l app.kubernetes.io/name=registry
    echo "Recent events:"
    kubectl get events --sort-by=.metadata.creationTimestamp
    echo "Pod logs:"
    kubectl logs -l app.kubernetes.io/name=registry --tail=50
    exit 1
  fi

  echo "Registry deployment successful! Showing recent logs:"
  kubectl logs -l app.kubernetes.io/name=registry --tail=20

  echo "Cleaning up temporary files..."
  rm -rf /tmp/registry-helm-config

  echo "Registry Helm chart successfully deployed"

}

uninstall_registry() {
  if helm ls --namespace default | grep -q '^registry\s'; then
      echo "Uninstalling Registry Helm chart..."
      helm uninstall registry
      kubectl delete secret registry-certs
      echo "Registry Helm chart successfully uninstalled"
  else
      echo "Registry Helm chart is not installed."
  fi
}

case $COMMAND in
  install)
    install_registry
    ;;
  uninstall)
    uninstall_registry
    ;;
  *)
    echo "Usage: $0 {install|uninstall}"
    exit 1
    ;;
esac