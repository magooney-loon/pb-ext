package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const specsDirEnv = "PB_EXT_OPENAPI_SPECS_DIR"
const disableEmbeddedSpecsEnv = "PB_EXT_DISABLE_EMBEDDED_OPENAPI_SPECS"
const testBinaryNameSuffix = ".test"

var (
	specsOnce     sync.Once
	specVersions  []string
	specsIndexErr error

	specsMu     sync.RWMutex
	parsedDocs  = make(map[string]*APIDocs)
	parseErrs   = make(map[string]error)
	specSources = make(map[string]string) // version -> "embed" | "disk"
)

// HasEmbeddedSpec returns true if a spec exists for the provided version.
// If PB_EXT_OPENAPI_SPECS_DIR is set, disk specs are preferred over embedded specs.
// If PB_EXT_DISABLE_EMBEDDED_OPENAPI_SPECS is truthy, this always returns false.
func HasEmbeddedSpec(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}

	if embeddedSpecsDisabled() {
		return false
	}

	source := specSourceFor(version)
	_, err := readSpecBytes(version, source)
	return err == nil
}

// ListEmbeddedSpecVersions returns discovered versions in stable sorted order.
// If PB_EXT_OPENAPI_SPECS_DIR is set, disk specs are preferred over embedded specs.
// If PB_EXT_DISABLE_EMBEDDED_OPENAPI_SPECS is truthy, this returns an empty list.
func ListEmbeddedSpecVersions() []string {
	if embeddedSpecsDisabled() {
		return []string{}
	}

	loadSpecsIndex()
	if specsIndexErr != nil {
		return []string{}
	}
	out := make([]string, len(specVersions))
	copy(out, specVersions)
	return out
}

// GetEmbeddedSpec loads a spec for version, caches parsed results, and returns a deep copy.
// If PB_EXT_OPENAPI_SPECS_DIR is set, disk specs are preferred over embedded specs.
// If PB_EXT_DISABLE_EMBEDDED_OPENAPI_SPECS is truthy, this returns a disabled error.
func GetEmbeddedSpec(version string) (*APIDocs, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}

	if embeddedSpecsDisabled() {
		return nil, fmt.Errorf("embedded openapi specs are disabled via %s", disableEmbeddedSpecsEnv)
	}

	// Read directly from source instead of relying on a separate existence pre-check.
	// This avoids stale/partial gating outcomes when the source is available but
	// index-based checks disagree.

	specsMu.RLock()
	if err, ok := parseErrs[version]; ok && err != nil {
		specsMu.RUnlock()
		return nil, err
	}
	if cached, ok := parsedDocs[version]; ok && cached != nil {
		cp, err := deepCopyAPIDocs(cached)
		specsMu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("failed to copy cached spec for version %q: %w", version, err)
		}
		return cp, nil
	}
	specsMu.RUnlock()

	source := specSourceFor(version)
	raw, err := readSpecBytes(version, source)
	if err != nil {
		parseErr := fmt.Errorf("failed to read %s spec for version %q: %w", source, version, err)
		specsMu.Lock()
		parseErrs[version] = parseErr
		specsMu.Unlock()
		return nil, parseErr
	}

	var docs APIDocs
	if err := json.Unmarshal(raw, &docs); err != nil {
		parseErr := fmt.Errorf("failed to parse %s spec for version %q: %w", source, version, err)
		specsMu.Lock()
		parseErrs[version] = parseErr
		specsMu.Unlock()
		return nil, parseErr
	}

	specsMu.Lock()
	parsedDocs[version] = &docs
	parseErrs[version] = nil
	specSources[version] = source
	specsMu.Unlock()

	cp, err := deepCopyAPIDocs(&docs)
	if err != nil {
		return nil, fmt.Errorf("failed to copy spec for version %q: %w", version, err)
	}
	return cp, nil
}

func loadSpecsIndex() {
	specsOnce.Do(func() {
		dir := specsDirPath()
		versions, err := listSpecVersionsFromDisk(dir)
		if err == nil && len(versions) > 0 {
			specVersions = versions
			return
		}

		// If PB_EXT_OPENAPI_SPECS_DIR was explicitly set, report the error
		if strings.TrimSpace(os.Getenv(specsDirEnv)) != "" {
			specsIndexErr = fmt.Errorf("failed to read specs directory %q: %w", dir, err)
			return
		}

		// No specs on disk and no explicit env - this is expected in dev mode
		// where specs are generated at runtime via AST parsing
		specVersions = []string{}
	})
}

func listSpecVersionsFromDisk(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		base := strings.TrimSuffix(name, ".json")
		if base == "" {
			continue
		}
		versions = append(versions, base)
	}

	sort.Strings(versions)
	return versions, nil
}

func specSourceFor(version string) string {
	specsMu.RLock()
	if source, ok := specSources[version]; ok && source != "" {
		specsMu.RUnlock()
		return source
	}
	specsMu.RUnlock()

	// Prefer disk by default (production). Only use env override.
	if strings.TrimSpace(os.Getenv(specsDirEnv)) != "" {
		return "disk"
	}
	// Default: disk (production) - runtime generation handles dev mode
	return "disk"
}

func readSpecBytes(version, source string) ([]byte, error) {
	switch source {
	case "disk":
		specPath := diskSpecPath(version)
		return os.ReadFile(specPath)
	default:
		return nil, fmt.Errorf("unknown spec source %q", source)
	}
}

func specsDirPath() string {
	if fromEnv := strings.TrimSpace(os.Getenv(specsDirEnv)); fromEnv != "" {
		return fromEnv
	}
	if strings.HasSuffix(filepath.Base(os.Args[0]), testBinaryNameSuffix) {
		return "specs"
	}
	return "specs"
}

func diskSpecPath(version string) string {
	return filepath.Join(specsDirPath(), version+".json")
}

func embeddedSpecsDisabled() bool {
	// During go test, compiled test binaries end with ".test".
	// Auto-disable embedded spec reads so unit tests exercise runtime generation paths
	// unless explicitly overridden via environment variable.
	if strings.HasSuffix(filepath.Base(os.Args[0]), testBinaryNameSuffix) {
		v := strings.TrimSpace(os.Getenv(disableEmbeddedSpecsEnv))
		if v == "" {
			return true
		}
	}

	v := strings.TrimSpace(os.Getenv(disableEmbeddedSpecsEnv))
	if v == "" {
		return false
	}

	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func deepCopyAPIDocs(in *APIDocs) (*APIDocs, error) {
	if in == nil {
		return nil, nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out APIDocs
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
