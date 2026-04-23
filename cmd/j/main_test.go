package main

import (
	"strings"
	"testing"

	"github.com/danielgtaylor/shorthand/v2"
)

func TestMarshalOutputTOMLNil(t *testing.T) {
	_, err := marshalOutput(nil, "toml")
	if err == nil {
		t.Fatal("expected an error for nil TOML input")
	}
	if !strings.Contains(err.Error(), "TOML only supports maps") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarshalOutputTOMLConvertsMapKeys(t *testing.T) {
	out, err := marshalOutput(map[any]any{1: "a"}, "toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `1 = "a"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestMarshalOutputShorthandMatchesLibrary(t *testing.T) {
	out, err := marshalOutput(map[string]any{"hello": "world"}, "shorthand")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := shorthand.MarshalPretty(map[string]any{"hello": "world"})
	if string(out) != expected {
		t.Fatalf("unexpected output: %q", out)
	}
}
