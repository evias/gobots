package driver

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSliceUnique tests order-preserving deduplication.
func TestSliceUnique(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "with_duplicates",
			input:    []string{"a", "b", "a", "c", "b", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "no_duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "all_duplicates",
			input:    []string{"x", "x", "x"},
			expected: []string{"x"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equalf(t, tc.expected, sliceUnique(tc.input),
				"expected %v, got %v", tc.expected, sliceUnique(tc.input))
		})
	}
}

// TestEnsureIncludePaths tests validation of --include directories.
func TestEnsureIncludePaths(t *testing.T) {
	dir := t.TempDir()

	// Valid directory passes.
	assert.NoError(t, ensureIncludePaths([]string{dir}))

	// Empty input passes.
	assert.NoError(t, ensureIncludePaths(nil))

	// Missing path fails.
	err := ensureIncludePaths([]string{filepath.Join(dir, "missing")})
	assert.ErrorContains(t, err, "--include")

	// A file (not a directory) fails.
	file := filepath.Join(dir, "file.yaml")
	require.NoError(t, os.WriteFile(file, []byte("name: test"), 0o600))

	err = ensureIncludePaths([]string{file})
	assert.ErrorContains(t, err, "not a directory")
}

// TestFindBotfiles tests recursive .yaml file discovery.
func TestFindBotfiles(t *testing.T) {
	root := t.TempDir()

	nested := filepath.Join(root, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	files := []string{
		filepath.Join(root, "robot.yaml"),
		filepath.Join(nested, "smartcar.yaml"),
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(f, []byte("name: test"), 0o600))
	}

	// Non-matching files are ignored.
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o600))
	// TODO: .yml files are not matched, only .yaml.
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.yml"), []byte("x"), 0o600))

	found := findBotfiles(root)

	assert.ElementsMatch(t, files, found)
}

// TestFindBotfiles_MissingRoot tests that a missing root yields no files.
func TestFindBotfiles_MissingRoot(t *testing.T) {
	found := findBotfiles(filepath.Join(t.TempDir(), "missing"))

	assert.Empty(t, found)
}

// TestInitLogs tests the debug log level toggle.
func TestInitLogs(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	// Without --debug, the default logger is left untouched.
	enableDebug = false
	initLogs()
	assert.Equal(t, orig, slog.Default())

	// With --debug, a new default logger is installed.
	enableDebug = true
	initLogs()
	assert.NotEqual(t, orig, slog.Default())

	enableDebug = false
}
