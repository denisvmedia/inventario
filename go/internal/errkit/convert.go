package errkit

import (
	"fmt"
	"strings"
)

func mapToString(m map[string]any) string {
	var strs []string
	for k, v := range m {
		strs = append(strs, fmt.Sprintf("%s=%v", k, v))
	}
	result := strings.Join(strs, ", ")

	return result
}
