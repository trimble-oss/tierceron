package utils

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

func TestHasInvalidAddrFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		invalid bool
	}{
		{name: "omitted", args: nil},
		{name: "https", args: []string{"-addr=https://vault.example.test"}},
		{name: "http", args: []string{"-addr=http://vault.example.test"}, invalid: true},
		{name: "token", args: []string{"-addr=token-value"}, invalid: true},
		{name: "empty", args: []string{"-addr="}, invalid: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flagset := flag.NewFlagSet(test.name, flag.ContinueOnError)
			flagset.String("addr", "", "")
			if err := flagset.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			if invalid := HasInvalidAddrFlag(flagset); invalid != test.invalid {
				t.Fatalf("HasInvalidAddrFlag() = %t, want %t", invalid, test.invalid)
			}
		})
	}
}

func TestRefMap(t *testing.T) {
	pluginParams := map[string]any{}
	pluginParams["testkey"] = "test"
	valRef := RefMap(pluginParams, "testkey")
	if *valRef != "test" {
		if valRef != nil {
			fmt.Fprintf(os.Stderr, "Expected 'test' value but got: %s\n", *valRef)
			t.Fatalf("Expected 'test' value but got: %s\n", *valRef)
		} else {
			fmt.Fprintf(os.Stderr, "Expected 'test' value but got: nil\n")
			t.Fatalf("Expected 'test' value but got: nil\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "Ref test 1 pass\n")
	}
	testVal := "testValRef"
	pluginParams["testkeyref"] = &testVal
	valRefRef := RefMap(pluginParams, "testkeyref")
	if *valRefRef != "testValRef" {
		if valRefRef != nil {
			fmt.Fprintf(os.Stderr, "Expected 'testValRef' value but got: %s\n", *valRefRef)
			t.Fatalf("Expected 'testValRef' value but got: %s\n", *valRefRef)
		} else {
			fmt.Fprintf(os.Stderr, "Expected 'testValRef' value but got: nil\n")
			t.Fatalf("Expected 'testValRef' value but got: nil\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "Ref test 2 pass\n")
	}
}
