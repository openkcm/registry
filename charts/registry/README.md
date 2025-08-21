# Registry Service Helm Chart

This Helm chart is used to deploy the Registry Service application. It includes configurations for the deployment, service, and other Kubernetes resources.

## Chart Components

- **Chart.yaml**: Contains metadata about the chart.
- **values.yaml**: Defines default values for the templates.
- **templates/**: Contains the Kubernetes resource templates.
- **_helpers.tpl**: Contains reusable template functions.

## Usage

To use this chart, you can override the default values in `values.yaml` by providing your own values file or using the `--set` flag with the `helm install` or `helm upgrade` commands.

### Example Usage

```sh
helm install my-release .
helm upgrade my-release . --set pod.replicas=3
helm uninstall my-release
```