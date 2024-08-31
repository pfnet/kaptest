package tester

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

type ResourceLoader struct {
	Vaps      map[string]*v1.ValidatingAdmissionPolicy
	Resources map[NameWithGVK]*unstructured.Unstructured
}

func NewResourceLoader() *ResourceLoader {
	return &ResourceLoader{
		Vaps:      map[string]*v1.ValidatingAdmissionPolicy{},
		Resources: map[NameWithGVK]*unstructured.Unstructured{},
	}
}

func (r *ResourceLoader) LoadVaps(paths []string) {
	for _, filePath := range paths {
		yamlFile, err := os.Open(filePath)
		if err != nil {
			slog.Error("read yaml file", "error", err)
			continue
		}
		decoder := kyaml.NewYAMLToJSONDecoder(yamlFile)
		for {
			var vap v1.ValidatingAdmissionPolicy
			if err := decoder.Decode(&vap); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				slog.Warn("failed to decode ValidatingAdmissionPolicy", "error", err)
				continue
			}
			if vap.Kind != "ValidatingAdmissionPolicy" {
				slog.Debug("skipped non-ValidatingAdmissionPolicy resource", "kind", vap.Kind, "name", vap.Name)
				continue
			}
			r.Vaps[vap.Name] = &vap
		}
	}
	for k := range r.Vaps {
		slog.Debug("ValidatingAdmissionPolicy laoded:", "name", k)
	}
}

func (r *ResourceLoader) LoadResources(paths []string) {
	for _, filePath := range paths {
		yamlFile, err := os.Open(filePath)
		if err != nil {
			slog.Error("read yaml file", "error", err)
			continue
		}
		decoder := kyaml.NewYAMLToJSONDecoder(yamlFile)
		for {
			var obj map[string]any
			if err := decoder.Decode(&obj); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				slog.Warn("failed to decode resource", "error", err)
				continue
			}
			unstructuredObj := &unstructured.Unstructured{Object: obj}
			ngvk := NewNameWithGVKFromObj(unstructuredObj)
			r.Resources[ngvk] = unstructuredObj
		}
	}
	for k := range r.Resources {
		slog.Debug("Resource loaded:", "name", k)
	}
}

func (r *ResourceLoader) GetResource(ngvk NameWithGVK) (*unstructured.Unstructured, error) {
	var obj *unstructured.Unstructured
	for k, v := range r.Resources {
		if k.Match(ngvk) {
			if obj != nil {
				return nil, fmt.Errorf("multiple target resource found: %+v", ngvk.String())
			}
			obj = v
		}
	}
	return obj, nil
}
