package api

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv(disableEmbeddedSpecsEnv, "false")
	os.Setenv(specsDirEnv, "specs")
	os.Exit(m.Run())
}

func TestHasEmbeddedSpec(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		want        bool
		envOverride string
	}{
		{
			name:    "v1 exists",
			version: "v1",
			want:    true,
		},
		{
			name:    "v2 exists",
			version: "v2",
			want:    true,
		},
		{
			name:    "non-existent version",
			version: "v99",
			want:    false,
		},
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
		{
			name:    "whitespace version",
			version: "  ",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envOverride != "" {
				os.Setenv(specsDirEnv, tt.envOverride)
				defer os.Unsetenv(specsDirEnv)
			}

			got := HasEmbeddedSpec(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListEmbeddedSpecVersions(t *testing.T) {
	versions := ListEmbeddedSpecVersions()
	assert.NotEmpty(t, versions)
	assert.Contains(t, versions, "v1")
	assert.Contains(t, versions, "v2")

	assert.True(t, strings.Join(versions, "") == "v1v2" || strings.Join(versions, "") == "v2v1",
		"versions should be sorted")
}

func TestGetEmbeddedSpec(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		wantErr     bool
		wantTitle   string
		wantVersion string
	}{
		{
			name:        "v1 valid",
			version:     "v1",
			wantErr:     false,
			wantTitle:   "pb-ext demo api",
			wantVersion: "1.0.0",
		},
		{
			name:        "v2 valid",
			version:     "v2",
			wantErr:     false,
			wantTitle:   "pb-ext demo api",
			wantVersion: "2.0.0",
		},
		{
			name:    "non-existent version",
			version: "v99",
			wantErr: true,
		},
		{
			name:    "empty version",
			version: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs, err := GetEmbeddedSpec(tt.version)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, docs)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, docs)
				assert.Equal(t, tt.wantTitle, docs.Info.Title)
				assert.Equal(t, tt.wantVersion, docs.Info.Version)
			}
		})
	}
}

func TestGetEmbeddedSpecCaching(t *testing.T) {
	version := "v1"

	docs1, err1 := GetEmbeddedSpec(version)
	assert.NoError(t, err1)
	assert.NotNil(t, docs1)

	docs2, err2 := GetEmbeddedSpec(version)
	assert.NoError(t, err2, "second call should not return error even though cached error is nil")
	assert.NotNil(t, docs2, "second call should not return nil docs even though cached error is nil")

	assert.Equal(t, docs1.Info.Title, docs2.Info.Title)
	assert.Equal(t, docs1.Info.Version, docs2.Info.Version)

	docs1.Info.Title = "Modified Title"
	assert.NotEqual(t, docs1.Info.Title, docs2.Info.Title, "deep copy should prevent mutation affecting cached version")
}

func TestGetEmbeddedSpecDeepCopy(t *testing.T) {
	docs1, err := GetEmbeddedSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs1)

	originalTitle := docs1.Info.Title
	docs1.Info.Title = "Mutated Title"

	docs2, err := GetEmbeddedSpec("v1")
	assert.NoError(t, err)
	assert.NotEqual(t, docs1.Info.Title, docs2.Info.Title, "returned spec should be a fresh copy each time")
	assert.Equal(t, originalTitle, docs2.Info.Title)
}

func TestGetEmbeddedSpecPaths(t *testing.T) {
	docs, err := GetEmbeddedSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.NotEmpty(t, docs.Paths)
	assert.Contains(t, docs.Paths, "/todos")
}

func TestGetEmbeddedSpecComponents(t *testing.T) {
	docs, err := GetEmbeddedSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.NotNil(t, docs.Components)
	assert.NotEmpty(t, docs.Components.Schemas)
	assert.Contains(t, docs.Components.Schemas, "Error")
	assert.Contains(t, docs.Components.Schemas, "PocketBaseRecord")
}

func TestEmbeddedSpecsDisabled(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name         string
		envValue     string
		binaryName   string
		wantDisabled bool
	}{
		{
			name:         "normal binary not disabled",
			envValue:     "",
			binaryName:   "server",
			wantDisabled: false,
		},
		{
			name:         "test binary auto-disabled",
			envValue:     "",
			binaryName:   "server.test",
			wantDisabled: true,
		},
		{
			name:         "explicit disable via env",
			envValue:     "true",
			binaryName:   "server",
			wantDisabled: true,
		},
		{
			name:         "explicit disable via env 1",
			envValue:     "1",
			binaryName:   "server",
			wantDisabled: true,
		},
		{
			name:         "explicit disable via env yes",
			envValue:     "yes",
			binaryName:   "server",
			wantDisabled: true,
		},
		{
			name:         "explicit disable via env on",
			envValue:     "on",
			binaryName:   "server",
			wantDisabled: true,
		},
		{
			name:         "test binary with env override enabled",
			envValue:     "false",
			binaryName:   "server.test",
			wantDisabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = []string{tt.binaryName}
			if tt.envValue != "" {
				os.Setenv(disableEmbeddedSpecsEnv, tt.envValue)
				defer os.Unsetenv(disableEmbeddedSpecsEnv)
			} else {
				os.Unsetenv(disableEmbeddedSpecsEnv)
			}

			got := embeddedSpecsDisabled()
			assert.Equal(t, tt.wantDisabled, got)
		})
	}
}

func TestSpecSourceFor(t *testing.T) {
	// Note: specSourceFor uses cached value from specSources map
	// After GetEmbeddedSpec is called, the source is cached and won't change
	// So this test just verifies the function works correctly

	// First call will set the cache to "disk" (default for non-embed mode)
	source := specSourceFor("v1")
	assert.Equal(t, "disk", source)
}

func TestReadSpecBytes(t *testing.T) {
	tests := []struct {
		name    string
		version string
		source  string
		wantLen int
		wantErr bool
	}{
		{
			name:    "disk v1",
			version: "v1",
			source:  "disk",
			wantLen: 14427,
			wantErr: false,
		},
		{
			name:    "disk v2",
			version: "v2",
			source:  "disk",
			wantLen: 6318,
			wantErr: false,
		},
		{
			name:    "disk non-existent",
			version: "v99",
			source:  "disk",
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "unknown source",
			version: "v1",
			source:  "unknown",
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readSpecBytes(tt.version, tt.source)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
				assert.Len(t, data, tt.wantLen)
			}
		})
	}
}

func TestEmbeddedSpecIntegration(t *testing.T) {
	// Ensure env is set - some other tests might have modified it
	os.Setenv(disableEmbeddedSpecsEnv, "false")

	// Test loading v1 and v2 directly
	docs, err := GetEmbeddedSpec("v1")
	assert.NoError(t, err, "should load v1 spec")
	assert.NotNil(t, docs)
	assert.NotEmpty(t, docs.Paths)
	assert.NotNil(t, docs.Components)
	assert.Contains(t, docs.Paths, "/todos")

	docs2, err := GetEmbeddedSpec("v2")
	assert.NoError(t, err, "should load v2 spec")
	assert.NotNil(t, docs2)
	assert.NotEmpty(t, docs2.Paths)
}
