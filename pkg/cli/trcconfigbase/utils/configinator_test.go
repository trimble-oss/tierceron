package utils

import (
	"fmt"
	"testing"
)

func TestGeneratePaths(t *testing.T) {
	_, _, err := generatePaths(nil)
	if err != nil {
		fmt.Println(err)
		t.Fatalf("Expected no error, got %v", err)
	}
}
