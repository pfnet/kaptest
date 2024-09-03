# Kaptest

**Kubernetes Admission Policy TESTing tool**

Kaptest is a testing tool to check the CEL expressions of [Validating Admission Policy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/).

Kaptest specializes in evaluating CEL expressions.
It allows you to perform fast and simple testing of CEL expressions,
without having to start the kube-apiserver, create the `ValidatingAdmissionPolicy` and parameter resources, or link it to the target resources using `ValidatingAdmissionPolicyBinding`.

## Installation

You can install Kaptest by downloading the compiled binary from the [release page](https://github.com/pfnet/kaptest/releases). It can also be installed by using the following script:

```shell
curl -sLO "https://github.com/pfnet/kaptest/releases/download/${KAPTEST_VERSION}/kaptest_${KAPTEST_VERSION}_${OS}_${ARCH}.tar.gz"
tar -xvf "kaptest_${KAPTEST_VERSION}_${OS}_${ARCH}.tar.gz"
```

## How to Use

### Setup Test Manifests

If you want to create a test for `./validatingadmissionpolicy.yaml`, run the following command:

```shell
kaptest init validatingadmissionpolicy.yaml
```

This will create a `./validatingadmissionpolicy.test` directory and generate a skeleton for writing test cases.

```shell
$ tree validationgadmissionpolicy.test/
validationgadmissionpolicy.test/
├── kaptest.yaml
└── resources.yaml

$ cat validationgadmissionpolicy.test/kaptest.yaml
validatingAdmissionPolicies:
- ../validationgadmissionpolicy.yaml
resources:
- resources.yaml
testSuites:
- policy: simple-policy
  tests:
  - object:
      kind: CHANGEME
      name: ok
    expect: admit
  - object:
      kind: CHANGEME
      name: bad
    expect: deny
```

### Test File Structures

There are no restrictions on the test file names. Although the skeleton created by `kaptest init` uses `kaptest.yaml`, other names can also be used.

Test files should be written in the following format:

```yaml
validatingAdmissionPolicies:
- <path/to/policy.yaml>
- <path/to/policy.yaml>
resources:
- <path/to/resource.yaml>
- <path/to/resource.yaml>
testSuites:
- policy: <name> # ValidatingAdmissionPolicy's name
  tests:
  - object:
      group: <group> # Optional
      version: <version> # Optional
      kind: <kind> # Required
      namespace: <namespace> # Optional: it is needed to match with namespaced resource
      name: <name> # Required
    oldObject:
      group: <group> # Optional
      version: <version> # Optional
      kind: <kind> # Required
      namespace: <namespace> # Optional
      name: <name> # Required
    params: # GVK of Params is omitted since it is defined by `spec.ParamKind` field in ValidatingAdmissionPolicy
      namespace: <namespace> # Optional
      name: <name> # Required
    userInfo: # The same struct as request.userInfo
      user: <sub>
      groups: <groups>
      extra: ...
    expect: <allow|deny|skip|error>
```

Resources specified in the `object`, `oldObject`, and `params` fields of the test cases, as well as `Namespace` resources, must be described in the YAML files specified in `resources` field.

### Run test

The tests defined in the above manifest can be run with the following command:

```shell
kaptest run <path/to/test_manifest.yaml> ...
```

You can specify multiple YAML files to display the test results together.

### Operation Type

You can describe the cases for CREATE, UPDATE, and DELETE operations based on whether object and oldObject are specified. These are determined by the following conditions:

- **CREATE**: Specify only object
- **UPDATE**: Specify both object and oldObject
- **DELETE**: Specify only oldObject

### Evaluation Results

Kaptest focuses on evaluating CEL expressions, so even when an error occurs or `matchConditions` are not met it does not change the result to `allow` or `deny`. The test results of Kaptest will be one of the following four values:

- **allow**: When all `matchConditions` and `validations` are evaluated as `true`
- **deny**: When all `matchConditions` are are evaluated as `true`, and at least one `validation` is evaluated as `false`
- **skip**: When at least one `matchCondition` is evaluated as `false`
- **error**: When at least one `matchCondition` or `validation` cannot be evaluated

Even if you configure the `spec.failurePolicy`, it will not affect the test results.

## Examples

Examples are [here](./examples/).

## Caveats

- Note that `matchExpressions` are never evaluated in this tool. This means you can specify any objects or oldObjects that do not satisfy `matchExpressions` in your test cases, though this is not recommended as it does not constitute a valid test case.


- The following [CEL variables](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/#validation-expression) are not supported for now.

  - `request.requestResource`
  - `request.subResource`
  - `request.requestSubResource`
  - `request.options`
  - `authorizer`

- The following attributes are fixed and cannot be changed.

  - `request.dryRun` = `True`
    - Since Kaptest is a testing tool, dryRun is always set to true.

## License

Copyright (c) 2024 Preferred Networks. All rights reserved. See [LICENSE](./LICENSE) for details.
