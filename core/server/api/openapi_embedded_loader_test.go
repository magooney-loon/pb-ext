package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv(disableSpecsEnv, "false")
	os.Setenv(specsDirEnv, filepath.Join("..", "..", "testutil", "specs"))
	os.Exit(m.Run())
}

func TestHasSpec(t *testing.T) {
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

			got := HasSpec(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListSpecVersions(t *testing.T) {
	versions := ListSpecVersions()
	assert.NotEmpty(t, versions)
	assert.Contains(t, versions, "v1")
	assert.Contains(t, versions, "v2")
}

func TestGetSpec(t *testing.T) {
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
			docs, err := GetSpec(tt.version)

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

func TestGetSpecCaching(t *testing.T) {
	version := "v1"

	docs1, err1 := GetSpec(version)
	assert.NoError(t, err1)
	assert.NotNil(t, docs1)

	docs2, err2 := GetSpec(version)
	assert.NoError(t, err2, "second call should not return error even though cached error is nil")
	assert.NotNil(t, docs2, "second call should not return nil docs even though cached error is nil")

	assert.Equal(t, docs1.Info.Title, docs2.Info.Title)
	assert.Equal(t, docs1.Info.Version, docs2.Info.Version)

	docs1.Info.Title = "Modified Title"

	assert.NotEqual(t, docs1.Info.Title, docs2.Info.Title, "deep copy should prevent mutation")
}

func TestGetSpecDeepCopy(t *testing.T) {
	docs1, err := GetSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs1)

	docs2, err := GetSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs2)

	docs1.Info.Title = "Modified Title"
	assert.NotEqual(t, docs1.Info.Title, docs2.Info.Title, "deep copy should prevent mutation")
}

func TestGetSpecPaths(t *testing.T) {
	docs, err := GetSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.NotEmpty(t, docs.Paths)
	assert.Contains(t, docs.Paths, "/todos")
}

func TestGetSpecComponents(t *testing.T) {
	docs, err := GetSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.NotNil(t, docs.Components)
}

func TestSpecsDisabled(t *testing.T) {
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
			envValue:     "false",
			binaryName:   "server.test",
			wantDisabled: false,
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
			os.Args[0] = tt.binaryName
			if tt.envValue != "" {
				os.Setenv(disableSpecsEnv, tt.envValue)
				defer os.Unsetenv(disableSpecsEnv)
			} else {
				os.Unsetenv(disableSpecsEnv)
			}

			got := specsDisabled()
			assert.Equal(t, tt.wantDisabled, got)
		})
	}
}

func TestReadSpecBytes(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantLen int
		wantErr bool
	}{
		{
			name:    "disk v1",
			version: "v1",
			wantLen: 330,
			wantErr: false,
		},
		{
			name:    "disk v2",
			version: "v2",
			wantLen: 330,
			wantErr: false,
		},
		{
			name:    "disk non-existent",
			version: "v99",
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readSpecBytes(tt.version)
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

func TestSpecIntegration(t *testing.T) {
	os.Setenv(disableSpecsEnv, "false")

	docs, err := GetSpec("v1")
	assert.NoError(t, err)
	assert.NotNil(t, docs)
	assert.NotEmpty(t, docs.Paths)
	assert.NotNil(t, docs.Components)
	assert.Contains(t, docs.Paths, "/todos")

	docs2, err := GetSpec("v2")
	assert.NoError(t, err)
	assert.NotNil(t, docs2)
	assert.NotEmpty(t, docs2.Paths)
}
