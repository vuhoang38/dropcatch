package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallBinaryCopiesIntoDataDir(t *testing.T) {
	src := filepath.Join(t.TempDir(), "dropcatch")
	if err := os.WriteFile(src, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	got, err := installBinary(src, data)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(data, "bin", "dropcatch"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	b, _ := os.ReadFile(got)
	st, _ := os.Stat(got)
	if string(b) != "v1" || st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("copy = %q mode %v", b, st.Mode())
	}

	// Reinstalling replaces it.
	_ = os.WriteFile(src, []byte("v2"), 0o755)
	if _, err := installBinary(src, data); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(got); string(b) != "v2" {
		t.Fatalf("after reinstall = %q", b)
	}
}
