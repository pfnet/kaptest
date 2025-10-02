package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pfnet/kaptest"
)

// get corresponding kubernetes veresion in form of "1.xx.yy" .
func GetSupportedKubernetesVersion() string {
	k8sioapiLine := ""
	for _, m := range strings.Split(kaptest.Gomod, "\n") {
		if strings.Contains(m, "k8s.io/api ") {
			k8sioapiLine = m
		}
	}
	if k8sioapiLine == "" {
		panic("k8s.io/api not found")
	}
	re := regexp.MustCompile(`v\d+\.\d+\.\d+$`)
	matches := re.FindStringSubmatch(k8sioapiLine)
	if len(matches) < 1 {
		panic(fmt.Errorf("unexpected format: '%s'", k8sioapiLine))
	}

	// convert v0.xx.yy to 1.xx.yy .
	return strings.Replace(matches[0], "v0", "1", 1)
}
