package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecGeneratorGenerateAndValidate(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewSpecGenerator(
		func() map[string]*APIDocsConfig {
			return map[string]*APIDocsConfig{
				"v1": {
					Title:   "Test API",
					Version: "1.0.0",
					BaseURL: "http://localhost:8080/api/v1",
				},
			}
		},
		nil,
	)

	err := gen.Generate(tmpDir, "")
	require.NoError(t, err)

	v1Path := filepath.Join(tmpDir, "v1.json")
	_, err = os.Stat(v1Path)
	require.NoError(t, err, "v1.json should be generated")

	err = gen.Validate(tmpDir)
	assert.NoError(t, err)
}

func TestValidateSpecs(t *testing.T) {
	tmpDir := t.TempDir()

	v1JSON := `{
		"openapi": "3.0.3",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {}
	}`

	err := os.WriteFile(filepath.Join(tmpDir, "v1.json"), []byte(v1JSON), 0644)
	require.NoError(t, err)

	err = ValidateSpecs(tmpDir, []string{"v1"})
	assert.NoError(t, err)
}

func TestValidateSpecsMissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	err := ValidateSpecs(tmpDir, []string{"v1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing or unreadable")
}

func TestValidateSpecsInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "v1.json"), []byte("not json"), 0644)
	require.NoError(t, err)

	err = ValidateSpecs(tmpDir, []string{"v1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestValidateSpecsMissingField(t *testing.T) {
	tmpDir := t.TempDir()

	v1JSON := `{
		"openapi": "3.0.3",
		"info": {
			"title": "Test API"
		},
		"paths": {}
	}`

	err := os.WriteFile(filepath.Join(tmpDir, "v1.json"), []byte(v1JSON), 0644)
	require.NoError(t, err)

	err = ValidateSpecs(tmpDir, []string{"v1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "info.version")
}
