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
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/yannh/kubeconform/pkg/resource"
	"github.com/yannh/kubeconform/pkg/validator"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

type ResourceLoader struct {
	Vaps      map[string]*v1.ValidatingAdmissionPolicy
	Maps      map[string]*v1alpha1.MutatingAdmissionPolicy
	Resources map[NameWithGVK]*unstructured.Unstructured
	validator validator.Validator
}

func NewResourceLoader(validator validator.Validator) *ResourceLoader {
	return &ResourceLoader{
		Vaps:      map[string]*v1.ValidatingAdmissionPolicy{},
		Maps:      map[string]*v1alpha1.MutatingAdmissionPolicy{},
		Resources: map[NameWithGVK]*unstructured.Unstructured{},
		validator: validator,
	}
}

func (r *ResourceLoader) LoadPolicies(paths []string) {
	for _, filePath := range paths {
		yamlFile, err := os.Open(filePath)
		if err != nil {
			slog.Error("read yaml file", "error", err)
			continue
		}
		yamlReader := kyaml.NewYAMLReader(bufio.NewReader(yamlFile))
		s := runtime.NewScheme()

		// supports admissionregistration.k8s.io/v1
		if err := v1.AddToScheme(s); err != nil {
			panic(fmt.Errorf("failed to add admissionregistration.k8s.io/v1 to scheme: %w", err))
		}
		// supports admissionregistration.k8s.io/v1alpha1 for MutatingAdmissionPolicy
		if err := v1alpha1.AddToScheme(s); err != nil {
			panic(fmt.Errorf("failed to add admissionregistration.k8s.io/v1alpha1 to scheme: %w", err))
		}
		decoder := serializer.NewCodecFactory(s).UniversalDeserializer()
		for {
			b, err := yamlReader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				slog.Warn("failed to read yaml file", "error", err)
				continue
			}

			if r.validator != nil {
				res := r.validator.ValidateResource(resource.Resource{Bytes: b})
				if res.Err != nil || res.Status != validator.Valid {
					slog.Error("Invalid policy exists")
					continue
				}
			}

			obj, gvk, err := decoder.Decode(b, nil, nil)
			if err != nil {
				slog.Warn("failed to decode policy", "error", err)
				continue
			}
			switch gvk.Kind {
			case "ValidatingAdmissionPolicy":
				if gvk.Version != "v1" {
					slog.Warn("only v1 ValidatingAdmissionPolicy is supported", "version", gvk.Version)
					continue
				}
				vap := obj.(*v1.ValidatingAdmissionPolicy)
				r.Vaps[vap.Name] = vap
			case "MutatingAdmissionPolicy":
				if gvk.Version != "v1alpha1" {
					slog.Warn("only v1alpha1 MutatingAdmissionPolicy is supported", "version", gvk.Version)
					continue
				}
				m := obj.(*v1alpha1.MutatingAdmissionPolicy)
				r.Maps[m.Name] = m
			default:
				slog.Warn("unexpected manifest", "kind", gvk.Kind)
			}
		}
	}
	for k := range r.Vaps {
		slog.Debug("ValidatingAdmissionPolicy laoded:", "name", k)
	}
	for k := range r.Maps {
		slog.Debug("MutatingAdmissionPolicy loaded:", "name", k)
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

			// if resource manifest validation is enabled, check whether the resource manifest follows a schema.
			if r.validator != nil {
				// TODO: avoid re-marshal
				objYamlBytes, err := yaml.Marshal(obj)
				if err != nil {
					slog.Warn("failed to marshal object into yaml", "error", err)
					continue
				}
				res := r.validator.ValidateResource(resource.Resource{Bytes: objYamlBytes})
				if res.Err != nil || res.Status != validator.Valid {
					slog.Error("A resource is invalid", "obj", unstructuredObj.GetName(), "status", res.Status, "error", res.Err)
					continue
				}
			}

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
		if ngvk.Match(k) {
			if obj != nil {
				return nil, fmt.Errorf("multiple target resource found: %+v", ngvk.String())
			}
			obj = v
		}
	}
	return obj, nil
}
