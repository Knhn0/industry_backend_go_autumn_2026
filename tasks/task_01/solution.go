package main

import (
	"fmt"
	"strings"
)

func greet(name string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "Hello, World!"
	}

	return fmt.Sprintf("Hello, %s!", trimmedName)
}
