/*
Copyright 2026 Preferred Networks, Inc.

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

package crd

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/controller/openapi/builder"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// Types contains CRD resource names and their shared schema converter.
type Types struct {
	Resources map[schema.GroupVersionKind]schema.GroupVersionResource
	Converter managedfields.TypeConverter
}

// NewTypes builds resource names and a schema converter from valid structural CRD schemas.
func NewTypes(crds []*apiextensionsv1.CustomResourceDefinition) (*Types, error) {
	types := &Types{Resources: make(map[schema.GroupVersionKind]schema.GroupVersionResource)}
	schemas := make(map[string]*spec.Schema)
	for _, definition := range crds {
		if definition == nil {
			return nil, fmt.Errorf("nil CRD definition")
		}
		crd := definition.DeepCopy()
		if crd.Spec.Group == "" || crd.Spec.Names.Kind == "" || crd.Spec.Names.Plural == "" {
			return nil, fmt.Errorf("CRD %q must specify group, kind and plural", crd.Name)
		}
		if crd.Spec.Names.ListKind == "" {
			crd.Spec.Names.ListKind = crd.Spec.Names.Kind + "List"
		}
		for _, version := range crd.Spec.Versions {
			if version.Name == "" {
				return nil, fmt.Errorf("CRD %s has an empty version %q", crd.Name, version.Name)
			}
			gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: version.Name, Kind: crd.Spec.Names.Kind}
			if _, exists := types.Resources[gvk]; exists {
				return nil, fmt.Errorf("duplicate resource kind %s", gvk)
			}
			if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
				return nil, fmt.Errorf("CRD %s version %s requires openAPIV3Schema", crd.Name, version.Name)
			}
			// Use Kubernetes' builder to retain merge extensions and
			// include the standard apiVersion, kind and ObjectMeta schemas and references.
			openapi, err := builder.BuildOpenAPIV3(crd, version.Name, builder.Options{})
			if err != nil {
				return nil, fmt.Errorf("build OpenAPI for %s: %w", gvk, err)
			}
			maps.Copy(schemas, openapi.Components.Schemas)
			types.Resources[gvk] = gvk.GroupVersion().WithResource(crd.Spec.Names.Plural)
		}
	}
	if len(schemas) > 0 {
		converter, err := managedfields.NewTypeConverter(schemas, false)
		if err != nil {
			return nil, fmt.Errorf("build CRD type converter: %w", err)
		}
		types.Converter = converter
	}
	return types, nil
}

// Load reads local files or HTTPS URLs without schema validation.
func Load(paths []string) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, path := range paths {
		definitions, err := loadSource(path)
		if err != nil {
			return nil, fmt.Errorf("load CRDs from %s: %w", path, err)
		}
		crds = append(crds, definitions...)
	}
	return crds, nil
}

func openSource(source string) (io.ReadCloser, error) {
	if !strings.HasPrefix(source, "https://") {
		if strings.Contains(source, "://") {
			return nil, fmt.Errorf("unsupported CRD URL scheme; use HTTPS")
		}
		return os.Open(source)
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("CRD redirects must use HTTPS")
			}
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	resp, err := client.Get(source)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("fetch CRD: HTTP %s", resp.Status)
	}
	return resp.Body, nil
}

func loadSource(source string) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	input, err := openSource(source)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	decoder := kyaml.NewYAMLToJSONDecoder(input)
	var crds []*apiextensionsv1.CustomResourceDefinition
	for {
		var crd *apiextensionsv1.CustomResourceDefinition
		if err := decoder.Decode(&crd); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if crd == nil {
			continue // Empty YAML document.
		}
		if crd.APIVersion != apiextensionsv1.SchemeGroupVersion.String() || crd.Kind != "CustomResourceDefinition" {
			return nil, fmt.Errorf("expected apiextensions.k8s.io/v1 CustomResourceDefinition, got %s %s", crd.APIVersion, crd.Kind)
		}
		crds = append(crds, crd)
	}
	if len(crds) == 0 {
		return nil, fmt.Errorf("source contains no CRD definitions")
	}
	return crds, nil
}
