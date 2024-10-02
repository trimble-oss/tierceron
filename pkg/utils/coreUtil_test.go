package utils

import (
	"fmt"
	"testing"
)

func TestRefMap(t *testing.T) {
	pluginParams := map[string]interface{}{}
	pluginParams["testkey"] = "test"
	valRef := RefMap(pluginParams, "testkey")
	if *valRef != "test" {
		if valRef != nil {
			fmt.Printf("Expected 'test' value but got: %s\n", *valRef)
			t.Fatalf("Expected 'test' value but got: %s\n", *valRef)
		} else {
			fmt.Printf("Expected 'test' value but got: nil\n")
			t.Fatalf("Expected 'test' value but got: nil\n")
		}
	} else {
		fmt.Printf("Ref test 1 pass\n")
	}
	testVal := "testValRef"
	pluginParams["testkeyref"] = &testVal
	valRefRef := RefMap(pluginParams, "testkeyref")
	if *valRefRef != "testValRef" {
		if valRefRef != nil {
			fmt.Printf("Expected 'testValRef' value but got: %s\n", *valRefRef)
			t.Fatalf("Expected 'testValRef' value but got: %s\n", *valRefRef)
		} else {
			fmt.Printf("Expected 'testValRef' value but got: nil\n")
			t.Fatalf("Expected 'testValRef' value but got: nil\n")
		}
	} else {
		fmt.Printf("Ref test 2 pass\n")
	}
}
