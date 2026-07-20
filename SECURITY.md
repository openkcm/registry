# Security Policy

## Reporting Security Vulnerabilities

If you discover a security vulnerability in this project, please report it responsibly. **Do not create public GitHub issues for security-related problems.**

Instead, please follow the instructions in our [security policy](https://github.com/openkcm/registry/security/policy) to report the vulnerability privately.

## Security Architecture

### Transport Security

This service is designed to be deployed behind [Linkerd](https://linkerd.io/), which provides:

- **Mutual TLS (mTLS)** between services in the mesh
- **Automatic certificate rotation**
- **Encrypted traffic** for all gRPC communications

> **Important:** The gRPC server does not implement TLS directly. It relies on the service mesh for transport-layer encryption. **Do not expose this service directly to untrusted networks without a properly configured service mesh or TLS termination proxy.**

### Authentication and Authorization

This service **does not implement authentication or authorization internally**. It is designed to operate behind an external authentication and authorization component that:

- Validates caller identity (authentication)
- Enforces access control policies (authorization)

> **Important:** Deploy this service only in environments where a properly configured authentication and authorization layer (e.g., an API gateway, service mesh with policy enforcement, or identity-aware proxy) validates all incoming requests before they reach this service.

## Security Considerations

### Database Security

- **Credentials**: Database credentials are never hardcoded. They are loaded from external sources (environment variables, files, or secret managers) via the `commoncfg.SourceRef` mechanism.
- **SQL Injection**: The service uses GORM with parameterized queries exclusively. No raw SQL string concatenation is performed with user input.
- **Connection Security**: Database connections are encrypted via Linkerd's mTLS when both the service and the database proxy are part of the service mesh.

### Input Validation

- All API inputs are validated using a configurable validation framework before processing.
- Field constraints (allowed values, patterns, non-empty requirements) are enforced at the service layer.
- Validation rules are defined in configuration and can be customized per deployment.

### Error Handling

- Panic recovery interceptors prevent crashes from propagating and ensure graceful degradation.
- Error messages returned to clients do not expose internal implementation details.
- Detailed error context is logged server-side for debugging without leaking sensitive information.

### Message Queue Security (Orbital)

For inter-region communication via AMQP:

- **mTLS authentication** is supported and recommended for message broker connections.
- Certificate files should be mounted from Kubernetes secrets, not embedded in configuration.
- Connection URLs should use `amqps://` (TLS-enabled) endpoints.

### Logging

- Structured logging (JSON format) is used throughout.
- Sensitive fields can be masked using the logger's PII masking configuration.
- Log levels should be set appropriately for production (avoid `debug` or `trace` in production).

## Deployment Security Checklist

Before deploying to production, verify:

- [ ] Service mesh (Linkerd) is properly configured with mTLS enabled
- [ ] External authentication/authorization is in place and validated
- [ ] Database credentials are sourced from a secret manager (not embedded)
- [ ] PostgreSQL is accessible via the service mesh (Linkerd mTLS)
- [ ] AMQP connections use mTLS (`amqps://` with certificate configuration)
- [ ] Log level is set to `info` or higher (not `debug` or `trace`)
- [ ] Network policies restrict access to only authorized services
- [ ] Kubernetes security context is configured (non-root user, read-only filesystem where possible)
- [ ] Pod security standards are enforced
- [ ] Resource limits are set to prevent DoS via resource exhaustion

## Dependency Management

- Dependencies are managed via Go modules with checksums verified.
- Dependabot is configured to automatically propose security updates.
- CodeQL analysis runs on all pull requests to detect potential vulnerabilities.

## Security Scanning

This repository employs multiple security scanning tools:

- **CodeQL**: Static analysis for security vulnerabilities in Go code
- **Dependabot**: Automated dependency vulnerability detection and updates
- **SonarQube**: Code quality and security analysis

## Secure Development Practices

Contributors should:

1. Never commit secrets, credentials, or API keys
2. Use parameterized queries for all database operations (GORM handles this)
3. Validate all external inputs before processing
4. Handle errors explicitly without exposing internal details
5. Follow the principle of least privilege for service accounts
6. Review security implications of all changes to authentication, authorization, or data handling code

## Version Support

Security updates are provided for the latest release version. We recommend always running the most recent version to ensure you have all security patches.
