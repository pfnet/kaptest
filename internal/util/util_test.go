package util

import (
	"testing"
)

func TestGetSupportedKubernetesVersion(t *testing.T) {
	if GetSupportedKubernetesVersion() != "1.36.3" {
		t.Fatalf("unexpected version")
	}
}
