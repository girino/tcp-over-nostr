package main

import (
	"fmt"
	"strings"
)

func isVersionCompatible(version string) bool {
	return strings.Contains(version, "2.0.") || strings.Contains(version, "v2.0.")
}

func main() {
	fmt.Println("Testing version compatibility:")
	fmt.Println("v2.0.1-version-compatibility-2-gcbddc8a-dirty:", isVersionCompatible("v2.0.1-version-compatibility-2-gcbddc8a-dirty"))
	fmt.Println("v2.0.0:", isVersionCompatible("v2.0.0"))
	fmt.Println("2.0.1:", isVersionCompatible("2.0.1"))
}
