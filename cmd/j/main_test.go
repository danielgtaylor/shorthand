package main

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

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

func TestMarshalOutputUnsupportedFormat(t *testing.T) {
	_, err := marshalOutput(map[string]any{"hello": "world"}, "ini")
	if err == nil {
		t.Fatal("expected an error for unsupported format")
	}
	if !strings.Contains(err.Error(), `unsupported format "ini"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarshalOutputTOMLRejectsNonMap(t *testing.T) {
	_, err := marshalOutput([]any{"hello"}, "toml")
	if err == nil {
		t.Fatal("expected an error for non-map TOML input")
	}
	if !strings.Contains(err.Error(), "TOML only supports maps") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fakeFileInfo struct {
	mode fs.FileMode
}

func (f fakeFileInfo) Name() string       { return "stdin" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeStdin struct {
	info os.FileInfo
	err  error
}

func (f fakeStdin) Stat() (os.FileInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.info, nil
}

func TestIsStdinPiped(t *testing.T) {
	piped, err := isStdinPiped(fakeStdin{info: fakeFileInfo{mode: 0}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !piped {
		t.Fatal("expected stdin to be treated as piped")
	}
}

func TestIsStdinPipedCharDevice(t *testing.T) {
	piped, err := isStdinPiped(fakeStdin{info: fakeFileInfo{mode: os.ModeCharDevice}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if piped {
		t.Fatal("expected stdin to be treated as not piped")
	}
}

func TestIsStdinPipedStatError(t *testing.T) {
	_, err := isStdinPiped(fakeStdin{err: errors.New("stat failed")})
	if err == nil {
		t.Fatal("expected stat error")
	}
	if !strings.Contains(err.Error(), "stat failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
