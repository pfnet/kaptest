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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestLoadAndNewTypes(t *testing.T) {
	definitions, err := Load([]string{"testdata/bundle.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 2 {
		t.Fatalf("expected two CRDs, got %d", len(definitions))
	}
	definition := definitions[0]
	original := definition.DeepCopy()
	types, err := NewTypes(definitions)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"v1", "v2"} {
		gvk := schema.GroupVersionKind{Group: "example.com", Version: version, Kind: "Shelf"}
		resource := types.Resources[gvk]
		if resource != gvk.GroupVersion().WithResource("shelves") || types.Converter == nil {
			t.Fatalf("incorrect CRD mapping for %s: %+v", gvk, resource)
		}
		object := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "example.com/" + version,
				"kind":       "Shelf",
				"metadata": map[string]any{
					"name": "bookshelf",
				},
				"spec": map[string]any{
					"parts": []any{
						map[string]any{
							"name":  "main",
							"value": "old",
						},
						map[string]any{
							"name":  "other",
							"value": "keep",
						},
					},
					"tags": []any{"old"},
				},
			},
		}
		patch := object.DeepCopy()
		patch.Object["spec"] = map[string]any{
			"parts": []any{
				map[string]any{
					"name":  "main",
					"value": "new",
				},
			},
			"tags": []any{"new"},
		}
		// Convert the original object using the CRD schema before merging.
		current, err := types.Converter.ObjectToTyped(object)
		if err != nil {
			t.Fatal(err)
		}
		// Convert the partial update using the same schema to preserve list merge rules.
		update, err := types.Converter.ObjectToTyped(patch)
		if err != nil {
			t.Fatal(err)
		}
		// Merge parts by name and replace atomic tags according to the CRD schema.
		merged, err := current.Merge(update)
		if err != nil {
			t.Fatal(err)
		}
		// Convert the merged value back to a Kubernetes object for comparison.
		got, err := types.Converter.TypedToObject(merged)
		if err != nil {
			t.Fatal(err)
		}
		want := object.DeepCopy()
		want.Object["spec"] = map[string]any{
			"parts": []any{
				map[string]any{
					"name":  "main",
					"value": "new",
				},
				map[string]any{
					"name":  "other",
					"value": "keep",
				},
			},
			"tags": []any{"new"},
		}
		// Verify main was updated, other was preserved, and tags were replaced.
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("schema merge mismatch for %s (-want +got):\n%s", version, diff)
		}
	}
	// Verify building the converter did not modify the supplied CRD definition.
	if diff := cmp.Diff(original, definition); diff != "" {
		t.Fatalf("input CRD was modified (-want +got):\n%s", diff)
	}
}

func TestNewTypesInvalidDefinition(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*apiextensionsv1.CustomResourceDefinition)
	}{
		{"missing schema", func(c *apiextensionsv1.CustomResourceDefinition) { c.Spec.Versions[0].Schema = nil }},
		{"duplicate version", func(c *apiextensionsv1.CustomResourceDefinition) {
			c.Spec.Versions = append(c.Spec.Versions, c.Spec.Versions[0])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definitions, err := Load([]string{"testdata/bundle.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			definition := definitions[0]
			tc.edit(definition)
			if _, err := NewTypes(definitions); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLoadSources(t *testing.T) {
	files := http.FileServer(http.Dir("testdata"))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/bundle.yaml", http.StatusFound)
		case "/downgrade":
			http.Redirect(w, r, "http://"+r.Host+"/bundle.yaml", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		default:
			files.ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	// Run sequentially and trust only the test server's certificate.
	originalTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	for _, tc := range []struct {
		name      string
		sources   []string
		count     int
		wantError string
	}{
		{"local", []string{"testdata/bundle.yaml"}, 2, ""},
		{"remote bundle", []string{server.URL + "/bundle.yaml"}, 2, ""},
		{"mixed", []string{"testdata/bundle.yaml", server.URL + "/bundle.yaml"}, 4, ""},
		{"redirect", []string{server.URL + "/redirect"}, 2, ""},
		{"downgrade", []string{server.URL + "/downgrade"}, 0, "redirects must use HTTPS"},
		{"redirect loop", []string{server.URL + "/loop"}, 0, "stopped after 10 redirects"},
		{"HTTP", []string{"http://example.com/crd.yaml"}, 0, "use HTTPS"},
		{"unsupported scheme", []string{"ftp://example.com/crd.yaml"}, 0, "use HTTPS"},
		{"invalid YAML", []string{"testdata/invalid.yaml"}, 0, "error converting YAML to JSON"},
		{"empty", []string{"testdata/empty.yaml"}, 0, "no CRD definitions"},
		{"wrong kind", []string{"testdata/wrong-kind.yaml"}, 0, "expected apiextensions.k8s.io/v1"},
		{"missing file", []string{"testdata/missing.yaml"}, 0, "load CRDs"},
		{"not found", []string{server.URL + "/missing.yaml"}, 0, "404"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definitions, err := Load(tc.sources)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected %q, got %v", tc.wantError, err)
				}
			} else if err != nil || len(definitions) != tc.count {
				t.Fatalf("expected %d CRDs, got %d: %v", tc.count, len(definitions), err)
			}
		})
	}
}
