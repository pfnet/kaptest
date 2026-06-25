package util

import (
	"testing"
)

func TestGetSupportedKubernetesVersion(t *testing.T) {
	if GetSupportedKubernetesVersion() != "1.35.3" {
		t.Fatalf("unexpected version")
	}
}
