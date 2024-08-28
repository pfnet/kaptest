# kaptest

## Overview

KAP Testing Tool is a testing tool for Kubernetes's:

- Validating Admission Policy

## CLI Usage

Examples are here: [examples/cli](./examples/cli)

### Installation

Install from [Releases](https://github.com/pfnet/kaptest/releases)

### Running

Define test suites in a YAML file.

```yaml
# manifest.yaml
validatingAdmissionPolicies:
  - policy.yaml
resources:
  - resources.yaml
testSuites:
  - policy: sample-policy
    tests:
      - object:
          kind: Pod
          name: bad-user
        expect: error
```

Run the tool with the test suites.

```bash
./kaptest ./manifest.yaml
```

## Library Usage

Examples are here: [examples/lib](./examples/lib)

```go
import "github.com/pfnet/kaptest"

func TestSimplePolicy(t *testing.T) {
	validator := validating.NewValidator(simplePolicy)
	result, _ := validator.Validate(validating.CelParams{Object: simpleDeployment})
	decision := result.Decisions[0]
	expectedResult := k8sValidating.EvalDeny
	if expectedResult != decision.Evaluation {
		t.Errorf("decision evaluation is expected to be %s, but got %s", expectedResult, decision.Evaluation)
	}
}
```

## Copyright

Copyright (c) 2024 Preferred Networks. See LICENSE for details.

