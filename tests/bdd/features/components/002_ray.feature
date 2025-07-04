Feature: Ray
  As a developer
  I want to test the Ray component lifecycle and operator reconciliation
  So that I can ensure Ray behaves correctly in managed and unmanaged states

  Background:
    Given the eventually timeout is `120s`
    Given the consistently timeout is `10s`

    # platform
    Given I set variable `dscName` to `test-dsc`

    # component
    Given I set variable `appNamespace` to `opendatahub`
    Given I set variable `componentName` to `ray`
    Given I set variable `componentKind` to `Ray`
    Given I set variable `componentInstance` to `default-ray`
    Given I set variable `deploymentName` to `kuberay-operator`

  Scenario: Setup
    Given I create the resource:
    """
    apiVersion: dscinitialization.opendatahub.io/v1
    kind: DSCInitialization
    metadata:
      name: {{.dsciName}}
    spec:
      applicationsNamespace: "{{.appNamespace}}"
      monitoring:
        managementState: "Removed"
      serviceMesh:
        managementState: "Removed"
      trustedCABundle:
        managementState: "Removed"
    """

    Given I create the resource:
    """
    apiVersion: datasciencecluster.opendatahub.io/v1
    kind: DataScienceCluster
    metadata:
      name: {{.dscName}}
    spec:
      components:
        ray:
          managementState: Removed
    """

  Scenario: Managed
    When I update the `dsc` `{{.dscName}}` with expression `.spec.components.{{.componentName}}.managementState = "Managed"`

    Then eventually the `ray` `{{.componentInstance}}` has conditions:
      | type     | status |
      | Ready    | True   |
    And eventually the `dsc` `{{.dscName}}` has conditions:
      | type     | status |
      | Ready    | True   |
      | RayReady | True   |
    And eventually the `rays.components.platform.opendatahub.io` `default-ray` should exist

  Scenario Outline: Resource Creation and Ownership <resourceType>

    And eventually `<resourceType>` in namespace `<namespace>` with selector `platform.opendatahub.io/part-of={{.componentName}}` should match expressions:
      | length > 0                                                                                                             |
      | .[0].metadata.ownerReferences[] \| select(.kind == "{{.componentKind}}" and .name == "{{.componentInstance}}") != null |

    Examples:
      | resourceType      | namespace         |
      | deployments       | {{.appNamespace}} |
      | serviceaccounts   | {{.appNamespace}} |
      | configmaps        | {{.appNamespace}} |
      | roles             | {{.appNamespace}} |
      | rolebindings      | {{.appNamespace}} |

  Scenario Outline: Cluster Resource Creation and Ownership <resourceType>
    And eventually `<resourceType>` with selector `platform.opendatahub.io/part-of={{.componentName}}` should match expressions:
      | length > 0                                                                                                             |
      | .[0].metadata.ownerReferences[] \| select(.kind == "{{.componentKind}}" and .name == "{{.componentInstance}}") != null |

    Examples:
      | resourceType        |
      | clusterroles        |
      | clusterrolebindings |

  Scenario: Scaling

    Given I set variable `requestsMemory` from `deployment` `{{.deploymentName}}` in namespace `{{.appNamespace}}` using expression `.spec.template.spec.containers[0].resources.requests.memory`

    When I update the `deployment` `{{.deploymentName}}` in namespace `{{.appNamespace}}` with expressions:
      | del(.metadata.annotations."opendatahub.io/managed")                   |
      | .spec.replicas = 2                                                    |
      | .spec.template.spec.containers[0].resources.requests.memory = "256Mi" |

    And consistently the `deployment` `{{.deploymentName}}` in namespace `{{.appNamespace}}` matches expressions:
      | .spec.replicas == 2                                                 |
      | .spec.template.spec.containers[0].resources.requests.memory == "256Mi" |

    When I update the `deployment` `{{.deploymentName}}` in namespace `{{.appNamespace}}` with expressions:
      | .metadata.annotations."opendatahub.io/managed" = "true" |

    Then eventually the `deployment` `{{.deploymentName}}` in namespace `{{.appNamespace}}` matches expressions:
      | .spec.replicas == 1                                                |
      | .spec.template.spec.containers[0].resources.requests.memory == "{{.requestsMemory}}" |

  Scenario: Removed
    When I update the `dsc` `{{.dscName}}` with expression `.spec.components.{{.componentName}}.managementState = "Removed"`
    Then eventually the `dsc` `{{.dscName}}` matches expression `.status.conditions[] | select(.type == "RayReady") | .status == "False"`
    And eventually the `dsc` `{{.dscName}}` has conditions:
      | type     | status |
      | Ready    | True   |
      | RayReady | False  |

    And eventually the `rays.components.platform.opendatahub.io` `{{.componentInstance}}` should not exist
    And eventually the `deployment` `{{.deploymentName}}` in namespace `{{.appNamespace}}` should not exist
    And eventually `deployments` in namespace `{{.appNamespace}}` with selector `platform.opendatahub.io/part-of={{.componentName}}` should match expression `length == 0`

    Scenario: Cleanup
      When I delete the `dsc` `{{.dscName}}`
      Then eventually the `dsc` `{{.dscName}}` should not exist
      
      When I delete the `dsci` `{{.dsciName}}`
      Then eventually the `dsci` `{{.dsciName}}` should not exist