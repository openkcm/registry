# OpenKCM Registry Helm chart

This chart deploys the [OpenKCM Registry](https://openkcm.io/components/cmk/registry):
the global registry of tenants and systems. The Registry exposes a gRPC API,
stores its data in PostgreSQL, and sends tenant jobs to the CMK of each region
over an AMQP broker.

Every value is documented in [values.yaml](values.yaml). Every setting of the
Registry itself, under `config`, is documented there with its default, its allowed
values and examples, and described in the
[Registry configuration reference](https://openkcm.io/components/cmk/security/registry-config).

## What the chart creates

| Resource | Purpose |
| --- | --- |
| Deployment | The Registry pods |
| ConfigMap | The Registry configuration, rendered from `config` and mounted as `/etc/registry/config.yaml` |
| Service | gRPC API (9092) and status endpoints (8888) inside the cluster |
| ServiceAccount | Identity of the pods. The Registry does not call the Kubernetes API, so no token is mounted |
| NetworkPolicy | Optional (`networkPolicy.enabled`): who may reach the pods |
| PodDisruptionBudget | Optional (`pod.disruptionBudget.enabled`) |
| HorizontalPodAutoscaler | Optional (`hpa.enabled`) |

## Prerequisites

- Kubernetes 1.23 or later (`autoscaling/v2`), and Helm 3 or later.
- A PostgreSQL database for the Registry, and a user that may create tables.
- An AMQP 1.0 broker, for example RabbitMQ, with addresses for each CMK region,
  and a client certificate for the Registry.
- The CMK tenant manager of each region, listening on the matching addresses.

## Install

Create Secrets for the database credentials and the broker certificates:

```sh
kubectl -n services create secret generic registry-database \
  --from-literal=username=registry --from-literal=password='…' \
  --from-file=ca.crt=postgres-ca.crt
kubectl -n services create secret generic registry-broker-certs \
  --from-file=tls.crt --from-file=tls.key --from-file=ca.crt
```

Create a values file, for example `registry-values.yaml`:

```yaml
# image.tag defaults to the chart's appVersion; set it only to deploy another version.

replicaCount: 2

extraVolumes:
  - name: db-credentials
    secret: { secretName: registry-database }
  - name: broker-certs
    secret: { secretName: registry-broker-certs }
extraVolumeMounts:
  - { name: db-credentials, mountPath: /etc/credentials/database, readOnly: true }
  - { name: broker-certs, mountPath: /etc/credentials/broker, readOnly: true }
extraEnvs:
  - { name: PGSSLMODE, value: verify-full }
  - { name: PGSSLROOTCERT, value: /etc/credentials/database/ca.crt }

networkPolicy:
  enabled: true
  grpc:
    from:
      - podSelector: { matchLabels: { app.kubernetes.io/name: cmk } }

config:
  database:
    host: postgresql.services.svc.cluster.local
    user:
      source: file
      file: { path: /etc/credentials/database/username, format: binary }
    password:
      source: file
      file: { path: /etc/credentials/database/password, format: binary }
  orbital:
    targets:
      - region: emea
        connection:
          type: amqp
          amqp:
            url: amqps://rabbitmq.services.svc.cluster.local:5671
            target: /queues/cmk.emea.tenants      # CMK tenantManager.amqp.source
            source: /queues/cmk.global.tenants    # CMK tenantManager.amqp.target
          auth:
            type: mtls
            mtls:
              certFile: /etc/credentials/broker/tls.crt
              keyFile: /etc/credentials/broker/tls.key
              caFile: /etc/credentials/broker/ca.crt
  # This list replaces the chart's default validations (Helm replaces lists), so it
  # repeats the four default rules and adds Tenant.Region.
  validations:
    - id: Auth.Type
      constraints: [{ type: list, spec: { allowList: ["oidc"] } }]
    - id: Auth.Properties
      constraints:
        - type: map-keys
          spec:
            keys: [{ name: issuer, required: true, constraints: [{ type: non-empty }] }]
    - id: Tenant.UserGroups
      constraints:
        - type: regex
          spec: { pattern: "^KMS_(TenantAdministrator|TenantAuditor)_[A-Za-z0-9-]+$" }
    - id: System.Type
      constraints: [{ type: list, spec: { allowList: ["system", "subaccount"] } }]
    - id: Tenant.Region
      constraints: [{ type: list, spec: { allowList: ["emea"] } }]
```

Install the chart:

```sh
helm upgrade --install registry ./charts/registry -n services -f registry-values.yaml
```

The release notes list warnings for settings that are not production-ready.

## Important values

| Value | Default | Description |
| --- | --- | --- |
| `image.tag` / `image.digest` | chart's `appVersion` | The Registry version to deploy. Leave both empty to deploy the version this chart release is made for |
| `image.repository` | `registry/images/registry` | Under `ghcr.io/openkcm` |
| `replicaCount` | `1` | Use two or more in production |
| `config.database.*` | placeholders | **Required.** PostgreSQL connection; load credentials from a Secret |
| `config.orbital.targets` | `[]` | **Required.** One entry per CMK region |
| `config.validations` | CMK-compatible rules | Restrict registered values; add your regions. A list you set replaces the defaults, so repeat the default rules |
| `networkPolicy.enabled` | `false` | **Recommended.** Restrict the gRPC API to its callers |
| `extraEnvs` | `[]` | For example `PGSSLMODE=verify-full` for TLS to PostgreSQL |
| `resources` | 100m CPU, 128Mi memory and 128Mi ephemeral storage requested; 512Mi memory and 512Mi ephemeral storage limits | |
| `hpa.enabled` | `false` | Autoscale on CPU and memory |
| `pod.disruptionBudget.enabled` | `false` | Limit voluntary disruptions |
| `configMap.immutable` | `false` | Immutable configuration, versioned by a content hash |

## Security defaults

| Setting | Default |
| --- | --- |
| Pod | User and group 65532 (non-root), `RuntimeDefault` seccomp profile, no ServiceAccount token, no Service environment variables |
| Container | No privilege escalation, all capabilities dropped, read-only root filesystem, writable `/tmp` |
| gRPC reflection | Off |
| Profiling endpoints | Off |
| Log level | `info` |

The Registry's gRPC API has no authentication of its own: anyone who can reach it
can create, block and terminate tenants. Keep the Service type `ClusterIP`,
enable `networkPolicy`, and run the Registry in a service mesh with mutual TLS.

## Network policy

With `networkPolicy.enabled: true`, the chart allows ingress only from the peers
in `networkPolicy.grpc.from` (gRPC API) and `networkPolicy.status.from` (status
port, for example a metrics collector). An empty list denies all callers of that
port. Outgoing traffic is not restricted. Probes from the kubelet are not
affected on most network plugins.

## Configuration changes

The Registry reads its configuration at startup. The chart adds a
`checksum/config` annotation to the pods, so a configuration change restarts
them during `helm upgrade`.

Everything under `config` is written to `config.yaml` (helper `registry.config` in
[templates/_helpers.tpl](templates/_helpers.tpl)), and the Registry rejects unknown keys at
startup. Keys that only control the chart, such as `configMap.immutable`, are outside
`config`.

With `configMap.immutable: true`, every configuration change creates a ConfigMap with a
new name and Helm deletes the previous one. Roll back with `helm rollback`, which
restores the previous ConfigMap. `kubectl rollout undo` does not work: the previous pods
mount the deleted ConfigMap and do not start.

## Upgrading from 1.3.x

| Change | Action |
| --- | --- |
| Default image repository is now `registry/images/registry`, so the default image is `ghcr.io/openkcm/registry/images/registry`, where release images are published. `image.tag` defaults to the chart's appVersion | If you set `image.repository` yourself, nothing changes. `image.tag` is no longer required |
| The ConfigMap is named after the chart's full name (`<fullname>-config`) instead of always `registry-config`, so two releases no longer collide. The full name is `<release>-registry`, or the release name if it contains `registry` | None; the chart switches the pods to the new ConfigMap. For a release named `registry` the name stays `registry-config` |
| `config.isImmutable` is no longer written into `config.yaml` | Prefer `configMap.immutable`. The old key still works |
| New security defaults (user and group 65532, read-only root filesystem, dropped capabilities) | The release image runs as this user (`nonroot`). Override `securityContext` and `pod.securityContext` only if a custom image needs another user |
| Probes and resource requests are set by default | Override `livenessProbe`, `readinessProbe`, `startupProbe` and `resources` if needed |
| `enableServiceLinks` is off | Set `pod.enableServiceLinks: true` if something in the pod needs Service variables |
| `config.grpcServer.flags.reflection` defaults to `false`, `config.logger.level` to `info`, `config.orbital.confirmJobAfter` to `2s` | Set them back explicitly if you rely on the old behaviour |
| Log records carry the level in the field `level` (`config.logger.formatter.fields.level: level`) instead of `info` | Update log queries that read the `info` field, or set `config.logger.formatter.fields.level: info` |
| The placeholder `validations` were replaced by rules that fit CMK | Review your `validations`; add `Tenant.Region` and others for your deployment |
| `image.command` accepts a list | A single string still works |
| PodDisruptionBudget without `minAvailable` or `maxUnavailable` uses `maxUnavailable: 1` | Set `minAvailable` explicitly if you want the old behaviour |

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Pod exits with `has invalid keys` | A key under `config` is not a Registry setting |
| Pod exits with `… must be greater than zero` | A required `config.orbital` value is missing |
| Pod exits at startup with an AMQP or certificate error | A broker in `config.orbital.targets` cannot be reached, or its `mtls` files are missing or invalid. The Registry connects to every target at startup |
| Pod never becomes ready | Database unreachable, wrong credentials or TLS settings, or `config.grpcServer.flags.health` is false |
| Tenants stay in a provisioning, blocking or terminating state | No CMK answers on the target of the tenant's region. After `config.orbital.maxPendingReconciles` sends without an answer, the tenant changes to an error state |
| Tenants change to an error state, for example `STATUS_PROVISIONING_ERROR`, soon after the request | No `config.orbital.targets` entry for the tenant's region |

## Uninstall

```sh
helm uninstall registry -n services
```

The database and its data are not removed.
