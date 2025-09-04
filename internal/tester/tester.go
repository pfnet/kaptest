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
	"github.com/yannh/kubeconform/pkg/validator"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var ErrTestFail = errors.New("test failed")

type TesterCmdConfig struct {
	CmdConfig
	ValidateResourceManifest bool
	SchemaCache              string
}

// Run runs the test cases defined in multiple manifest files.
func Run(cfg TesterCmdConfig, pathList []string) error {
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
func runEach(cfg TesterCmdConfig, manifestPath string) testResultSummary {
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

	var manifestValidator validator.Validator = nil
	if cfg.ValidateResourceManifest {
		// Below line causes gofumpt's false positive
		err = os.MkdirAll(cfg.SchemaCache, 0755) //nolint:gofumpt
		if err != nil {
			return testResultSummary{
				manifestPath: manifestPath,
				fail:         1,
				message:      fmt.Sprintf("FAIL: create manifest validator schema cache: %v", err),
			}
		}
		validatorOpts := validator.Opts{
			Cache:                cfg.SchemaCache,
			Debug:                cfg.Debug,
			SkipTLS:              false,
			SkipKinds:            map[string]struct{}{},
			RejectKinds:          map[string]struct{}{},
			KubernetesVersion:    "1.32.1", // ensure matching a version with validation.go and mutation.go
			Strict:               true,
			IgnoreMissingSchemas: false,
		}
		schemaLocations := []string{"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}/{{ .ResourceKind }}{{ .KindSuffix }}.json"}
		schemaLocations = append(schemaLocations, manifests.SchemaLocations...)
		manifestValidator, err = validator.New(schemaLocations, validatorOpts)
		if err != nil {
			return testResultSummary{
				manifestPath: manifestPath,
				fail:         1,
				message:      fmt.Sprintf("FAIL: create manifest validator: %v", err),
			}
		}
	}

	// Load Policies and other resources
	loader := NewResourceLoader(manifestValidator)
	loader.LoadPolicies(manifests.Policies)
	loader.LoadResources(manifests.Resources)

	results := []testResult{}

	// Run test cases for VAP one by one
	for _, tt := range manifests.VapTestSuites {
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
				results = append(results, newSetupErrorResult(tt.Policy, tc, errs))
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
			validationResult := validator.Validate(given)

			results = append(results, newVAPEvalResult(tt.Policy, tc, validationResult.Decisions))
		}
	}

	// Run test cases for MAP one by one
	for _, tt := range manifests.MapTestSuites {
		policy, ok := loader.Maps[tt.Policy]
		if !ok {
			results = append(results, newPolicyNotFoundResult(tt.Policy))
			continue
		}

		mutator, err := kaptest.NewMutator(policy)
		if err != nil {
			panic(err)
		}
		for _, tc := range tt.Tests {
			slog.Debug("SETUP: ", "policy", tt.Policy, "expect", tc.Expect, "object", tc.Object.String(), "oldObject", tc.OldObject.String(), "expectObject", tc.ExpectObject.String())

			given, expectedObj, errs := newMutationParams(policy, tc, loader)
			if errs != nil {
				results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, errs))
				continue
			}

			matchedHooks, err := mutator.EvalMatchCondition(given)
			if err != nil {
				results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, []error{err}))
				continue
			}

			// if there is no matched hooks, it is considered to be not matched
			if len(matchedHooks) == 0 {
				results = append(results, newPolicyNotMatchConditionResult(tt.Policy, tc, "no matched hooks"))
				continue
			}

			// check match results with hooks
			matched := true
			for _, h := range matchedHooks {
				if h.Result.Error != nil {
					results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, []error{h.Result.Error}))
					matched = false
					break
				}
				if !h.Result.Matches {
					results = append(results, newPolicyNotMatchConditionResult(tt.Policy, tc, h.Result.FailedConditionName))
					matched = false
					break
				}
			}
			if !matched {
				continue
			}

			slog.Debug("RUN:   ", "policy", tt.Policy, "expect", tc.Expect, "object", tc.Object.String(), "oldObject", tc.OldObject.String(), "expectOjbect", tc.ExpectObject.String())
			mutatedObj, err := mutator.Mutate(given)
			if err != nil {
				results = append(results, newPolicyEvalErrorResult(tt.Policy, tc, []error{err}))
			} else {
				results = append(results, newMAPEvalResult(tt.Policy, tc, matchedHooks, expectedObj, mutatedObj))
			}
		}
	}

	return summarize(manifestPath, results, cfg.Verbose)
}

func newValidationParams(vap *v1.ValidatingAdmissionPolicy, tc VAPTestCase, loader *ResourceLoader) (kaptest.ValidationParams, []error) {
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
	if vap.Spec.ParamKind != nil {
		paramGVK := schema.FromAPIVersionAndKind(vap.Spec.ParamKind.APIVersion, vap.Spec.ParamKind.Kind)
		if paramObj, err = getParamObj(loader, paramGVK, tc.Param); err != nil {
			errs = append(errs, fmt.Errorf("get param: %w", err))
		}
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

func newMutationParams(mp *v1alpha1.MutatingAdmissionPolicy, tc MAPTestCase, loader *ResourceLoader) (kaptest.MutationParams, runtime.Object, []error) {
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
	var expectObj *unstructured.Unstructured
	if tc.Expect == Mutate { //nolint
		if !tc.ExpectObject.IsValid() {
			errs = append(errs, fmt.Errorf("expectObject must be given when mutate expected"))
		} else {
			if expectObj, err = loader.GetResource(tc.ExpectObject); err != nil {
				errs = append(errs, fmt.Errorf("get expectObject: %w", err))
			}
			if expectObj == nil {
				errs = append(errs, fmt.Errorf("expectObject must be given when mutate expected"))
			} else if !tc.DisableNameOverwrite {
				expectObj.SetName(obj.GetName())
			}
		}
	}

	var paramObj *unstructured.Unstructured
	if mp.Spec.ParamKind != nil {
		paramGVK := schema.FromAPIVersionAndKind(mp.Spec.ParamKind.APIVersion, mp.Spec.ParamKind.Kind)
		paramObj, err = getParamObj(loader, paramGVK, tc.Param)
		if err != nil {
			errs = append(errs, fmt.Errorf("get param: %w", err))
		}
	}

	var namespaceObj *corev1.Namespace
	if namespaceObj, err = getNamespaceObj(loader, obj, oldObj); err != nil {
		errs = append(errs, fmt.Errorf("get namespace: %w", err))
	}

	userInfo := NewK8sUserInfo(tc.UserInfo)

	// We need to ensure the object follows scheme
	// by converting unstructured object into typed object
	// TODO: support CRD
	objs := []*unstructured.Unstructured{obj, oldObj, paramObj, expectObj}
	typedObjs := []runtime.Object{nil, nil, nil, nil}
	for idx, o := range objs {
		if o == nil {
			continue
		}
		typedObjs[idx], err = convertToTyped(o)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to convert %s to typed object: %w", o.GetObjectKind().GroupVersionKind(), err))
		}
	}

	if len(errs) > 0 {
		return kaptest.MutationParams{}, nil, errs
	}

	param := kaptest.MutationParams{
		Object:       typedObjs[0],
		OldObject:    typedObjs[1],
		ParamObj:     typedObjs[2],
		NamespaceObj: namespaceObj,
		UserInfo:     &userInfo,
	}

	return param, typedObjs[3], nil
}

func getParamObj(loader *ResourceLoader, paramGVK schema.GroupVersionKind, param NamespacedName) (*unstructured.Unstructured, error) {
	if param.Name == "" {
		return nil, fmt.Errorf("param name is empty")
	}

	paramNGVK := NewNameWithGVK(paramGVK, param)
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
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
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

func convertToTyped(obj *unstructured.Unstructured) (runtime.Object, error) {
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to register scheme: %w", err)
	}

	gvk := obj.GroupVersionKind()
	newTypedObject, err := scheme.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("GVK %s is not registered in the scheme: %w", gvk, err)
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, newTypedObject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to typed object: %w", err)
	}

	return newTypedObject, nil
}
