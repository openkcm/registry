#!/bin/bash

######################################################################
# This script manages the installation and uninstallation of Helm charts
# for PostgreSQL, RabbitMQ, and OpenTelemetry Collector in a Kubernetes cluster.
# It checks for prerequisites, deploys necessary configurations, and ensures
# that the services are set up correctly.
######################################################################

set -e

# Configuration
POSTGRES_CHART_VERSION="16.7.26"
RABBITMQ_CHART_VERSION="16.0.14"
OTEL_COLLECTOR_CHART_VERSION="0.74.0"
DB_USER="postgres"
DB_PASS="secret"
DB_NAME="registry"

# Parameters
COMMAND=$1

#######################################################################
install_postgres() {
  if helm ls --namespace default | grep -q '^postgresql\s'; then
      echo "PostgreSQL Helm chart is already installed."
      return 0
  fi

  echo "Deploying PostgreSQL Helm chart..."
  helm upgrade --install postgresql bitnami/postgresql \
  		--version ${POSTGRES_CHART_VERSION} \
  		--set auth.username=${DB_USER} \
  		--set auth.password=${DB_PASS} \
  		--set auth.database=${DB_NAME} \
  		--set resources.limits.cpu="1000m" \
  		--set resources.limits.memory="1Gi" \
  		--set resources.requests.cpu="250m" \
  		--set resources.requests.memory="256Mi" \
  		--wait --timeout=60s

  echo "PostgreSQL Helm chart successfully deployed"
}

uninstall_postgres() {
  if helm ls --namespace default | grep -q '^postgresql\s'; then
      echo "Uninstalling PostgreSQL Helm chart..."
      helm uninstall postgresql
      echo "PostgreSQL Helm chart successfully uninstalled"
  else
      echo "PostgreSQL Helm chart is not installed."
  fi
}

install_rabbitmq() {
  if helm ls --namespace default | grep -q '^rabbitmq\s'; then
      echo "RabbitMQ Helm chart is already installed."
      return 0
  fi

  echo "Creating RabbitMQ configuration..."
  mkdir -p /tmp/rabbitmq-helm-config
  cp local/rabbitmq/definitions.json /tmp/rabbitmq-helm-config/
  cp local/rabbitmq/rabbitmq.conf /tmp/rabbitmq-helm-config/
  cp local/rabbitmq/enabled_plugins /tmp/rabbitmq-helm-config/
  cp -r local/rabbitmq/certs /tmp/rabbitmq-helm-config/

  kubectl create secret generic rabbitmq-definitions \
      --from-file=load_definition.json=/tmp/rabbitmq-helm-config/definitions.json \
      --dry-run=client -o yaml | kubectl apply -f -

  kubectl create secret generic rabbitmq-certs \
      --from-file=ca.crt=/tmp/rabbitmq-helm-config/certs/ca.crt \
      --from-file=tls.crt=/tmp/rabbitmq-helm-config/certs/server.crt \
      --from-file=tls.key=/tmp/rabbitmq-helm-config/certs/server.key \
      --dry-run=client -o yaml | kubectl apply -f -

  cat > /tmp/rabbitmq-helm-config/values.yaml << 'EOF'
  auth:
    tls:
      enabled: true
      existingSecret: rabbitmq-certs
      sslOptionsVerify: verify_peer
      failIfNoPeerCert: true
  image:
    debug: true

  service:
    type: ClusterIP
    ports:
      amqp: 5672
      amqpTls: 5671
      manager: 15672

  persistence:
    enabled: false

  plugins: "rabbitmq_auth_mechanism_ssl rabbitmq_management"

  loadDefinition:
    enabled: true
    existingSecret: rabbitmq-definitions

  extraConfiguration: |
    load_definitions = /app/load_definition.json

    # Listen on plain AMQP
    listeners.tcp = none

    # Listen on SSL/TLS
    listeners.ssl.default = 5671

    # Map client cert CN → RabbitMQ user
    ssl_cert_login_from = common_name

    auth_mechanisms.1 = EXTERNAL

    log.console.level = debug

  resources:
    limits:
      memory: 512Mi
      cpu: 500m
    requests:
      memory: 256Mi
      cpu: 100m
EOF

  echo "Deploying RabbitMQ Helm chart..."
  helm upgrade --install rabbitmq bitnami/rabbitmq \
      --values /tmp/rabbitmq-helm-config/values.yaml \
      --version ${RABBITMQ_CHART_VERSION} \
      --wait --timeout=300s

  echo "Cleaning up temporary files..."
  rm -rf /tmp/rabbitmq-helm-config

  echo "RabbitMQ Helm chart successfully deployed"
}

uninstall_rabbitmq() {
  if helm ls --namespace default | grep -q '^rabbitmq\s'; then
      echo "Uninstalling RabbitMQ Helm chart..."
      helm uninstall rabbitmq
      kubectl delete secret rabbitmq-definitions
      kubectl delete secret rabbitmq-certs
      echo "RabbitMQ Helm chart successfully uninstalled"
  else
      echo "RabbitMQ Helm chart is not installed."
  fi
}

install_otel_collector() {
  if helm ls --namespace default | grep -q '^opentelemetry-collector\s'; then
      echo "OpenTelemetry Collector Helm chart is already installed."
      return 0
  fi

  echo "Deploying OpenTelemetry Collector Helm chart..."
  # Deploy OpenTelemetry Collector Helm chart
  helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
  helm upgrade --install opentelemetry-collector open-telemetry/opentelemetry-collector \
      --version ${OTEL_COLLECTOR_CHART_VERSION} \
      --set mode=deployment \
      --set config.receivers.otlp.protocols.grpc.endpoint="0.0.0.0:4317" \
      --set config.exporters.logging.loglevel="debug" \
      --set config.service.pipelines.traces.receivers="{otlp}" \
      --set config.service.pipelines.traces.exporters="{debug}" \
      --set config.service.pipelines.metrics.receivers="{otlp}" \
      --set config.service.pipelines.metrics.exporters="{debug}" \
      --set config.service.pipelines.logs.receivers="{otlp}" \
      --set config.service.pipelines.logs.exporters="{debug}" \
      --set resources.limits.cpu="500m" \
      --set resources.limits.memory="512Mi" \
      --set resources.requests.cpu="100m" \
      --set resources.requests.memory="128Mi" \
      --wait --timeout=60s

  echo "OpenTelemetry Collector Helm chart successfully deployed"
}

uninstall_otel_collector() {
  if helm ls --namespace default | grep -q '^opentelemetry-collector\s'; then
      echo "Uninstalling OpenTelemetry Collector Helm chart..."
      helm uninstall opentelemetry-collector
      echo "OpenTelemetry Collector Helm chart successfully uninstalled"
  else
      echo "OpenTelemetry Collector Helm chart is not installed."
  fi
}

prerequisites() {
  if ! command -v helm &> /dev/null
  then
      echo "Helm could not be found, please install it first."
      exit 1
  fi

  if ! command -v kubectl &> /dev/null
  then
      echo "kubectl could not be found, please install it first."
      exit 1
  fi

  if ! helm repo list | grep -q 'bitnami'; then
      echo "Adding Bitnami Helm repository..."
      helm repo add bitnami https://charts.bitnami.com/bitnami
      helm repo update
  fi
}

install() {
  prerequisites
  install_postgres
  install_rabbitmq
  install_otel_collector
}

uninstall() {
  uninstall_postgres
  uninstall_rabbitmq
  uninstall_otel_collector
}

case $COMMAND in
  install)
    install
    ;;
  uninstall)
    uninstall
    ;;
  *)
    echo "Usage: $0 {install|uninstall}"
    exit 1
    ;;
esac
