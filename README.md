# kaptest

## Overview

KAP Testing Tool is a testing tool for Kubernetes's:

- Validating Admission Policy

## CLI Usage

Examples are here: [examples/cli](./examples/cli)

### Installation

Install from [Releases](https://github.com/pfnet/kaptest/releases)

### Running

Move to a directory to create `.kaptest/` directory and run `kaptest init`.
Then `kaptest.yaml` and `resources.yaml` will be created in `.kaptest/`

```bash
$ cd /path/to/dir
$ kaptest init .
```

Define test suites in a YAML file.

```yaml
# .kaptest/kaptest.yaml
validatingAdmissionPolicies:
  - ../policy.yaml
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
kaptest run .kaptest/kaptest.yaml
```

## Library Usage

Examples are here: [examples/lib](./examples/lib)

```go
import "github.com/pfnet/kaptest"

func TestSimplePolicy(t *testing.T) {
	validator := kaptest.NewValidator(simplePolicy)
	result, _ := validator.Validate(kaptest.CelParams{Object: simpleDeployment})
	decision := result.Decisions[0]
	expectedResult := validating.EvalDeny
	if expectedResult != decision.Evaluation {
		t.Errorf("decision evaluation is expected to be %s, but got %s", expectedResult, decision.Evaluation)
	}
}
```

## Copyright

Copyright (c) 2024 Preferred Networks. See LICENSE for details.
