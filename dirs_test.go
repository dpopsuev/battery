package battery_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/battery"
)

func TestDataDir_EnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	dir, err := battery.DataDir("myapp")
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	want := filepath.Join(tmp, "myapp")
	if dir != want {
		t.Errorf("got %s, want %s", dir, want)
	}
	assertDirExists(t, dir)
}

func TestConfigDir_EnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := battery.ConfigDir("myapp")
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	want := filepath.Join(tmp, "myapp")
	if dir != want {
		t.Errorf("got %s, want %s", dir, want)
	}
	assertDirExists(t, dir)
}

func TestCacheDir_EnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dir, err := battery.CacheDir("myapp")
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	want := filepath.Join(tmp, "myapp")
	if dir != want {
		t.Errorf("got %s, want %s", dir, want)
	}
	assertDirExists(t, dir)
}

func TestDataDir_Fallback(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", t.TempDir())

	dir, err := battery.DataDir("testapp")
	if err != nil {
		t.Fatalf("DataDir fallback: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "share", "testapp")
	if dir != want {
		t.Errorf("got %s, want %s", dir, want)
	}
	assertDirExists(t, dir)
}

func TestDataDir_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	dir1, err := battery.DataDir("myapp")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	dir2, err := battery.DataDir("myapp")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if dir1 != dir2 {
		t.Errorf("not idempotent: %s != %s", dir1, dir2)
	}
}

func TestDataDir_Permissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	dir, err := battery.DataDir("secure")
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o700 {
		t.Errorf("permissions: got %o, want 700", perm)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("directory does not exist: %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("not a directory: %s", path)
	}
}
