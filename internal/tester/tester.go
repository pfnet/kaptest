/*
Copyright 2024 Preferred Networks, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tester

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pfnet/kaptest"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var ErrTestFail = errors.New("test failed")

// Run runs the test cases defined in multiple manifest files.
func Run(cfg CmdConfig, pathList []string) error {
	var passCount, failCount int
	for _, path := range pathList {
		r := runEach(cfg, path)
		fmt.Println(r.String(false))
		passCount += r.pass
		failCount += r.fail
	}

	if len(pathList) > 1 {
		fmt.Println("--------------------------------------------------")
		fmt.Printf("Total: %d, Pass: %d, Fail: %d\n", passCount+failCount, passCount, failCount)
	}

	if failCount > 0 {
		return ErrTestFail
	}
	return nil
}

// runEach runs the test cases defined in a single manifest file.
func runEach(cfg CmdConfig, manifestPath string) testResultSummary {
	// Read manifest yaml
	manifestFile, err := os.ReadFile(manifestPath)
	if err != nil {
		return testResultSummary{
			manifestPath: manifestPath,
			fail:         1,
			message:      fmt.Sprintf("FAIL: read manifest YAML: %v", err),
		}
	}

	var manifests TestManifests
	if err := yaml.Unmarshal(manifestFile, &manifests); err != nil {
		return testResultSummary{
			manifestPath: manifestPath,
			fail:         1,
			message:      fmt.Sprintf("FAIL: unmarshal manifest YAML: %v", err),
		}
	}
	if ok, msg := manifests.IsValid(); !ok {
		return testResultSummary{
			manifestPath: manifestPath,
			fail:         1,
			message:      fmt.Sprintf("FAIL: invalid manifest: %v", msg),
		}
	}

	// Change directory to the base directory of manifest
	pwd, err := os.Getwd()
	if err != nil {
		return testResultSummary{
			manifestPath: manifestPath,
			fail:         1,
			message:      fmt.Sprintf("FAIL: get current directory: %v", err),
		}
	}
	if err := os.Chdir(filepath.Dir(manifestPath)); err != nil {
		return testResultSummary{
			manifestPath: manifestPath,
			fail:         1,
			message:      fmt.Sprintf("FAIL: change directory: %v", err),
		}
	}
	defer os.Chdir(pwd) //nolint:errcheck

	// Load validatingAdmissionPolicies and other resources
	loader := NewResourceLoader()
	loader.LoadVaps(manifests.ValidatingAdmissionPolicies)
	loader.LoadResources(manifests.Resources)

	results := []testResult{}

	// Run test cases one by one
	for _, tt := range manifests.TestSuites {
		// Create Validator
		vap, ok := loader.Vaps[tt.Policy]
		if !ok {
			results = append(results, newPolicyNotFoundResult(tt.Policy))
			continue
		}
		validator := kaptest.NewValidator(vap)

		for _, tc := range tt.Tests {
			slog.Debug("SETUP: ", "policy", tt.Policy, "expect", tc.Expect, "object", tc.Object.String(), "oldObject", tc.OldObject.String(), "param", tc.Param.String())

			// Setup params for validation
			given, errs := newValidationParams(vap, tc, loader)
			if len(errs) > 0 {
				results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, errs))
				continue
			}

			// Run EvalMatchConditions
			if vap.Spec.MatchConditions != nil {
				matchResult := validator.EvalMatchCondition(given)
				if matchResult.Error != nil {
					results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, []error{matchResult.Error}))
					continue
				}
				if !matchResult.Matches {
					results = append(results, newPolicyNotMatchConditionResult(tt.Policy, tc, matchResult.FailedConditionName))
					continue
				}
			}
			// Run validation
			slog.Debug("RUN:   ", "policy", tt.Policy, "expect", tc.Expect, "object", tc.Object.String(), "oldObject", tc.OldObject.String(), "param", tc.Param.String())
			validationResult, err := validator.Validate(given)
			if err != nil {
				results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, []error{err}))
				continue
			}

			results = append(results, newPolicyEvalResult(tt.Policy, tc, validationResult.Decisions))
		}
	}

	return summarize(manifestPath, results, cfg.Verbose)
}

func newValidationParams(vap *v1.ValidatingAdmissionPolicy, tc TestCase, loader *ResourceLoader) (kaptest.ValidationParams, []error) {
	var errs []error
	var err error
	var obj, oldObj *unstructured.Unstructured
	if !tc.Object.IsValid() && !tc.OldObject.IsValid() {
		errs = append(errs, fmt.Errorf("object or oldObject must be given and valid"))
	} else {
		if obj, err = loader.GetResource(tc.Object); err != nil {
			errs = append(errs, fmt.Errorf("get object: %w", err))
		}
		if oldObj, err = loader.GetResource(tc.OldObject); err != nil {
			errs = append(errs, fmt.Errorf("get oldObject: %w", err))
		}
		if obj == nil && oldObj == nil {
			errs = append(errs, fmt.Errorf("neither object nor oldObject found"))
		}
	}

	var paramObj *unstructured.Unstructured
	if paramObj, err = getParamObj(loader, vap, tc.Param); err != nil {
		errs = append(errs, fmt.Errorf("get param: %w", err))
	}

	var namespaceObj *corev1.Namespace
	if namespaceObj, err = getNamespaceObj(loader, obj, oldObj); err != nil {
		errs = append(errs, fmt.Errorf("get namespace: %w", err))
	}

	userInfo := NewK8sUserInfo(tc.UserInfo)

	if len(errs) > 0 {
		return kaptest.ValidationParams{}, errs
	}

	return kaptest.ValidationParams{
		Object:       obj,
		OldObject:    oldObj,
		ParamObj:     paramObj,
		NamespaceObj: namespaceObj,
		UserInfo:     &userInfo,
	}, nil
}

func getParamObj(loader *ResourceLoader, vap *v1.ValidatingAdmissionPolicy, param NamespacedName) (*unstructured.Unstructured, error) {
	if vap.Spec.ParamKind == nil {
		return nil, nil
	}
	if param.Name == "" {
		return nil, fmt.Errorf("param name is empty")
	}

	paramNGVK := NewNameWithGVK(schema.FromAPIVersionAndKind(vap.Spec.ParamKind.APIVersion, vap.Spec.ParamKind.Kind), param)
	paramObj, err := loader.GetResource(paramNGVK)
	if err != nil {
		return nil, fmt.Errorf("get param: %w", err)
	}
	if paramObj == nil {
		return nil, fmt.Errorf("param not found")
	}
	return paramObj, nil
}

func getNamespaceObj(loader *ResourceLoader, obj, oldObj *unstructured.Unstructured) (*corev1.Namespace, error) {
	if obj == nil && oldObj == nil {
		return nil, fmt.Errorf("neither object nor oldObject found")
	}
	namespaceName, err := getNamespaceName(obj, oldObj)
	if err != nil {
		return nil, fmt.Errorf("extract namespace: %w", err)
	}
	if namespaceName == "" {
		return nil, nil
	}

	namespaceNGVK := NewNameWithGVK(schema.FromAPIVersionAndKind("v1", "Namespace"), NamespacedName{Name: namespaceName})
	uNamespaceObj, err := loader.GetResource(namespaceNGVK)
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}
	if uNamespaceObj == nil {
		slog.Info("use default namespace with no labels and annotations", "namespace", namespaceName)
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}, nil
	}

	var namespaceObj corev1.Namespace
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uNamespaceObj.Object, &namespaceObj); err != nil {
		return nil, fmt.Errorf("convert to namespace: %w", err)
	}
	return &namespaceObj, nil
}

func getNamespaceName(obj, oldObj *unstructured.Unstructured) (string, error) {
	if oldObj == nil {
		return obj.GetNamespace(), nil
	}
	if obj == nil {
		return oldObj.GetNamespace(), nil
	}
	if obj.GetNamespace() != oldObj.GetNamespace() {
		return "", errors.New("namespace is different between object and oldObject")
	}
	return obj.GetNamespace(), nil
}
