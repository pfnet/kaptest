package tester

import (
	"errors"
	"fmt"
	"kaptest"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ErrTestFail = errors.New("test failed")
)

// Run runs the test cases defined in the manifest file.
func Run(cfg CmdConfig, manifestPath string) error {
	// Read manifest yaml
	manifestFile, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest YAML: %w", err)
	}

	var manifests TestManifests
	if err := yaml.Unmarshal(manifestFile, &manifests); err != nil {
		return fmt.Errorf("unmarshal manifest YAML: %w", err)
	}

	// Change directory to the base directory of manifest
	if err := os.Chdir(filepath.Dir(manifestPath)); err != nil {
		return fmt.Errorf("change directory: %w", err)
	}

	// Load validatingAdmissionPolicies
	loader := NewResourceLoader()
	loader.LoadVaps(manifests.ValidatingAdmissionPolicies)
	loader.LoadResources(manifests.Resources)
	loader.LoadParams(manifests.Params)
	loader.LoadNamespaces(manifests.Namespaces)

	results := []TestResult{}

	// Run test cases one by one
	for _, tt := range manifests.TestSuites {
		// Create Validator
		vap, ok := loader.Vaps[tt.Policy]
		if !ok {
			results = append(results, NewPolicyNotFoundResult(tt.Policy))
			continue
		}
		validator := kaptest.NewValidator(*vap)

		for _, tc := range tt.Tests {
			slog.Debug("SETUP: ", "policy", tt.Policy, "expect", tc.Expect, "object", tc.Object.String(), "oldObject", tc.OldObject.String(), "param", tc.Param.String())

			// Setup params for validation
			given, errs := newValidationParams(vap, tc, loader)
			if len(errs) > 0 {
				results = append(results, NewPolicyEvalErrorResult(tt.Policy, tc, errs))
				continue
			}

			// Run validation
			slog.Debug("RUN:   ", "policy", tt.Policy, "expect", tc.Expect, "object", tc.Object.String(), "oldObject", tc.OldObject.String(), "param", tc.Param.String())
			validationResult, err := validator.Validate(given)
			if err != nil {
				results = append(results, NewPolicyEvalErrorResult(tt.Policy, tc, []error{err}))
				continue
			}

			results = append(results, newPolicyEvalResult(tt.Policy, tc, validationResult.Decisions))
		}
	}

	// Show results
	out, pass := Summarize(results, cfg.Verbose)
	fmt.Println(out)

	if !pass {
		return ErrTestFail
	}
	return nil
}

func newValidationParams(vap *v1.ValidatingAdmissionPolicy, tc TestCase, loader *ResourceLoader) (kaptest.CelParams, []error) {
	var errs []error
	var object, oldObject *unstructured.Unstructured
	if !tc.Object.IsValid() && !tc.OldObject.IsValid() {
		errs = append(errs, fmt.Errorf("object or oldObject must be given and valid"))
	} else {
		for k, v := range loader.Resources {
			if k.Match(tc.Object) {
				if object != nil {
					errs = append(errs, fmt.Errorf("multiple target resource found for object: %+v", tc.Object))
					break
				}
				object = v
			}
			if k.Match(tc.OldObject) {
				if oldObject != nil {
					errs = append(errs, fmt.Errorf("multiple target resource found for oldObject: %+v", tc.OldObject))
					break
				}
				oldObject = v
			}
		}
	}

	var paramObj *unstructured.Unstructured
	if vap.Spec.ParamKind != nil {
		if tc.Param.Name != "" {
			for k, v := range loader.Params {
				paramNGVK := NewNameWithGVK(schema.FromAPIVersionAndKind(vap.Spec.ParamKind.APIVersion, vap.Spec.ParamKind.Kind), tc.Param)
				if k.Match(paramNGVK) {
					if paramObj != nil {
						errs = append(errs, fmt.Errorf("multiple target resource found for param: %+v", tc.Param))
					}
					paramObj = v
				}
			}
			if paramObj == nil {
				errs = append(errs, fmt.Errorf("param not found"))
			}
		}
	}

	var namespaceObj *corev1.Namespace
	if object == nil && oldObject == nil {
		errs = append(errs, fmt.Errorf("neither object nor oldObject found"))
	} else {
		namespaceName, err := getNamespaceName(object, oldObject)
		if err != nil {
			errs = append(errs, fmt.Errorf("extract namespace: %w", err))
		} else if namespaceName != "" {
			var ok bool
			namespaceObj, ok = loader.Namespaces[namespaceName]
			if !ok {
				errs = append(errs, fmt.Errorf("namespace not found"))
			}
		}
	}

	userInfo := NewUserInfo(tc.UserInfo)

	if len(errs) > 0 {
		return kaptest.CelParams{}, errs
	}
	return kaptest.CelParams{
		Object:       object,
		OldObject:    oldObject,
		ParamObj:     paramObj,
		NamespaceObj: namespaceObj,
		UserInfo:     &userInfo,
	}, nil
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
