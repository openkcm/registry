[![REUSE status](https://api.reuse.software/badge/github.com/openkcm/registry)](https://api.reuse.software/info/github.com/openkcm/registry)


# Registry

## About this project

Registry is the central data management service for the CMK landscape. It manages CMK tenants and systems.

Tenants hold information about CMK tenants like owner id and type and region.
It is used by
* Tenant Management API to create, block and terminate tenants,
* regional CMK layer to implement the tenant lifecycle used by the CMK application.

Systems are all customer-exposed business tenants of any kind.
The system resource holds information about the system, the crypto layer region and key assignment information.
It is used by
* Crypto layer to announce or terminate systems,
* CMK application to manage information about the systems and their key assignment.


## Requirements and Setup

### Dependencies

* [Go 1.25.0+](https://golang.org/)
* [gorm](https://github.com/go-gorm/gorm)
* [Docker](https://www.docker.com/)
* [Docker Compose](https://docs.docker.com/compose/)

### Running the Registry

The registry depends on a PostgreSQL database, RabbitMQ for message queuing and
OpenTelemetry Collector for metrics collection. You can run these dependencies using Docker Compose.

```sh
make docker-compose-dependencies-up
```

If you also want to see the Docker logs, instead run:

```sh
make docker-compose-dependencies-up-and-log
```

Then, run the registry itself:

```sh
go run ./cmd/registry/main.go
```

Alternatively, you can use Docker Compose to run the registry along with its dependencies, also 
including Grafana for metrics visualization. 

You can access the Grafana dashboards via the  
URL http://localhost:3004. Use `admin/admin` for login and look for existing dashboards.
```sh
make docker-compose-up
```

If you also want to see the Docker logs, instead run

```shell
make docker-compose-up-and-log
```

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openkcm/registry/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openkcm/registry/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/openkcm/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright (20xx-)20xx SAP SE or an SAP affiliate company and OpenKCM contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openkcm/registry).
