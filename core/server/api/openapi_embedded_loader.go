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
const disableSpecsEnv = "PB_EXT_DISABLE_OPENAPI_SPECS"
const testBinaryNameSuffix = ".test"

var (
	specsOnce     sync.Once
	specVersions  []string
	specsIndexErr error

	specsMu    sync.RWMutex
	parsedDocs = make(map[string]*APIDocs)
	parseErrs  = make(map[string]error)
)

// HasSpec returns true if a spec exists for the provided version on disk.
// If PB_EXT_OPENAPI_SPECS_DIR is set, specs are read from that directory.
// If PB_EXT_DISABLE_OPENAPI_SPECS is truthy, this always returns false.
func HasSpec(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}

	if specsDisabled() {
		return false
	}

	_, err := readSpecBytes(version)
	return err == nil
}

// ListSpecVersions returns discovered versions in stable sorted order from disk.
// If PB_EXT_OPENAPI_SPECS_DIR is set, specs are read from that directory.
// If PB_EXT_DISABLE_OPENAPI_SPECS is truthy, this returns an empty list.
func ListSpecVersions() []string {
	if specsDisabled() {
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

// GetSpec loads a spec for version from disk, caches parsed results, and returns a deep copy.
// If PB_EXT_OPENAPI_SPECS_DIR is set, specs are read from that directory.
// If PB_EXT_DISABLE_OPENAPI_SPECS is truthy, this returns a disabled error.
func GetSpec(version string) (*APIDocs, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}

	if specsDisabled() {
		return nil, fmt.Errorf("openapi specs are disabled via %s", disableSpecsEnv)
	}

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

	raw, err := readSpecBytes(version)
	if err != nil {
		parseErr := fmt.Errorf("failed to read spec for version %q: %w", version, err)
		specsMu.Lock()
		parseErrs[version] = parseErr
		specsMu.Unlock()
		return nil, parseErr
	}

	var docs APIDocs
	if err := json.Unmarshal(raw, &docs); err != nil {
		parseErr := fmt.Errorf("failed to parse spec for version %q: %w", version, err)
		specsMu.Lock()
		parseErrs[version] = parseErr
		specsMu.Unlock()
		return nil, parseErr
	}

	specsMu.Lock()
	parsedDocs[version] = &docs
	parseErrs[version] = nil
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

		if strings.TrimSpace(os.Getenv(specsDirEnv)) != "" {
			specsIndexErr = fmt.Errorf("failed to read specs directory %q: %w", dir, err)
			return
		}

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

func readSpecBytes(version string) ([]byte, error) {
	specPath := diskSpecPath(version)
	return os.ReadFile(specPath)
}

func specsDirPath() string {
	if fromEnv := strings.TrimSpace(os.Getenv(specsDirEnv)); fromEnv != "" {
		return fromEnv
	}
	return "specs"
}

func diskSpecPath(version string) string {
	return filepath.Join(specsDirPath(), version+".json")
}

func specsDisabled() bool {
	if strings.HasSuffix(filepath.Base(os.Args[0]), testBinaryNameSuffix) {
		v := strings.TrimSpace(os.Getenv(disableSpecsEnv))
		if v == "" {
			return true
		}
	}

	v := strings.TrimSpace(os.Getenv(disableSpecsEnv))
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
