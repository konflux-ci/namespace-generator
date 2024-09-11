# namespace-generator Plugin for ArgoCD ApplicationSet

The `namespace-generator` plugin is designed to work with ArgoCD ApplicationSets to generate ArgoCD Applications based on Kubernetes namespaces. This plugin allows you to define conditions for selecting namespaces and will automatically generate an ArgoCD Application for each namespace that meets the specified criteria.

## Features

- **Namespace Filtering**: List Kubernetes namespaces based on specific conditions defined in the ApplicationSet resource.
- **Automatic Application Generation**: Create ArgoCD Applications for each namespace that matches the conditions.
- **Local Cluster Support**: Currently supports only the local Kubernetes cluster.

## Core Use Case

A key use case for the `namespace-generator` plugin is to deploy a default set of Kubernetes resources to every new namespace that meets certain filtering criteria. This can be particularly useful in scenarios where you want to ensure that all namespaces have a consistent set of resources, such as default configurations, monitoring tools, quotas, or security policies.

## Installation

1. **Install ArgoCD**: Follow the [ArgoCD installation guide](https://argo-cd.readthedocs.io/en/stable/getting_started/).
2. **Install the namespace-generator plugin**:
This installation guide assumes that `ArgoCD`, including the `ApplicationSet` controller are installed in the the `argocd` namespace.

**NOTE:** For production deployment don't
forget to replace the value of the secret.

```bash
kubectl create -k manifests
```

## Example Configuration

1. **Define the ApplicationSet Resource**: Create an `ApplicationSet` resource that uses the `namespace-generator` plugin to list and filter namespaces. Below is a sample configuration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: example-app
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
    - plugin:
        configMapRef:
          name: namespace-generator-plugin
        input:
          parameters:
            labelSelector:
              matchLabels:
                konflux.ci/type: user
              matchExpressions:
                - key: app.kubernetes.io/instance
                  operator: DoesNotExist
        requeueAfterSeconds: 30
  syncPolicy:
    # To be on the safe side, prevent an Application's child resources from being deleted,
    # when the parent Application is deleted
    preserveResourcesOnDeletion: true
  template:
    metadata:
      name: "{{ .namespace }}-default-app"
    spec:
      project: default
      source:
        path: example/resources
        repoURL: https://github.com/gbenhaim/namespace-generator
        targetRevision: main
      destination:
        namespace: '{{.namespace}}'
        server: https://kubernetes.default.svc
      syncPolicy:
        automated:
          prune: false
          selfHeal: false
        retry:
          limit: 10
          backoff:
            duration: 10s
            factor: 2
            maxDuration: 3m

```

2. **Apply the Configuration**: Apply the `ApplicationSet` resource to your cluster.

    ```sh
    kubectl apply -f your-applicationset-definition.yaml
    ```

## ApplicationSet Plugin Documentation

For more detailed information on how to use ApplicationSet plugins, please refer to the official [ApplicationSet Plugin Documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Plugin/).

## Troubleshooting

- Ensure that your `ApplicationSet` YAML is correctly formatted and contains valid plugin arguments.
- Verify that the namespaces in your cluster meet the filter criteria defined in the `ApplicationSet`.

## Contributing

If you have any suggestions or improvements, please feel free to contribute by submitting a pull request or opening an issue.

## License

See the [LICENSE](LICENSE) file for details.
