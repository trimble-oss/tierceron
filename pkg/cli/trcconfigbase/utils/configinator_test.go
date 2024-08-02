package utils

import (
	"fmt"
	"testing"
)

func TestGeneratePaths(t *testing.T) {
	_, _, err := generatePaths(nil)
	if err == nil {
		fmt.Printf("Expected nil config error, got %s\n", err)
		t.Fatalf("Expected nil config error, got %v", err)
	}
}
