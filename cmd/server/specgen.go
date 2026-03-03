package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/magooney-loon/pb-ext/core/server/api"
)

const openAPISpecsDirEnv = "PB_EXT_OPENAPI_SPECS_DIR"
const disableEmbeddedSpecsEnv = "PB_EXT_DISABLE_EMBEDDED_OPENAPI_SPECS"

func generateSpecs(outputDir string, onlyVersion string) error {
	if outputDir == "" {
		return fmt.Errorf("spec output directory is required")
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create specs directory %q: %w", outputDir, err)
	}

	originalSpecsDirEnv, hadSpecsDirEnv := os.LookupEnv(openAPISpecsDirEnv)
	if err := os.Unsetenv(openAPISpecsDirEnv); err != nil {
		return fmt.Errorf("failed to unset %s for generation: %w", openAPISpecsDirEnv, err)
	}
	originalDisableEmbeddedEnv, hadDisableEmbeddedEnv := os.LookupEnv(disableEmbeddedSpecsEnv)
	if err := os.Setenv(disableEmbeddedSpecsEnv, "1"); err != nil {
		return fmt.Errorf("failed to set %s for generation: %w", disableEmbeddedSpecsEnv, err)
	}
	defer func() {
		if hadSpecsDirEnv {
			_ = os.Setenv(openAPISpecsDirEnv, originalSpecsDirEnv)
		} else {
			_ = os.Unsetenv(openAPISpecsDirEnv)
		}

		if hadDisableEmbeddedEnv {
			_ = os.Setenv(disableEmbeddedSpecsEnv, originalDisableEmbeddedEnv)
		} else {
			_ = os.Unsetenv(disableEmbeddedSpecsEnv)
		}
	}()

	versionManager := initVersionedSystem()
	if err := registerVersionedRoutesForDocsGeneration(versionManager); err != nil {
		return fmt.Errorf("failed to register routes for docs generation: %w", err)
	}

	versions := versionManager.GetAllVersions()
	sort.Strings(versions)

	if onlyVersion != "" {
		found := false
		for _, v := range versions {
			if v == onlyVersion {
				found = true
				versions = []string{onlyVersion}
				break
			}
		}
		if !found {
			return fmt.Errorf("version %q is not configured", onlyVersion)
		}
	}

	for _, version := range versions {
		registry, err := versionManager.GetVersionRegistry(version)
		if err != nil {
			return fmt.Errorf("failed to get registry for version %q: %w", version, err)
		}

		docs := registry.GetDocsWithComponents()
		if docs == nil {
			return fmt.Errorf("generated docs are nil for version %q", version)
		}

		specPath := filepath.Join(outputDir, version+".json")
		data, err := json.MarshalIndent(docs, "", "\t")
		if err != nil {
			return fmt.Errorf("failed to marshal spec for version %q: %w", version, err)
		}

		if err := os.WriteFile(specPath, data, 0o644); err != nil {
			return fmt.Errorf("failed to write spec file %q: %w", specPath, err)
		}
	}

	return validateGeneratedVersions(outputDir, versions)
}

func validateSpecs(specsDir string) error {
	if specsDir == "" {
		return fmt.Errorf("specs directory is required")
	}

	versionManager := initVersionedSystem()
	versions := versionManager.GetAllVersions()
	sort.Strings(versions)

	return validateGeneratedVersions(specsDir, versions)
}

func validateGeneratedVersions(specsDir string, versions []string) error {
	if len(versions) == 0 {
		return fmt.Errorf("no configured API versions found")
	}

	for _, version := range versions {
		specPath := filepath.Join(specsDir, version+".json")
		if err := validateSingleSpecFile(specPath, version); err != nil {
			return err
		}
	}

	return nil
}

func validateSingleSpecFile(specPath string, expectedVersion string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("missing or unreadable spec file %q: %w", specPath, err)
	}

	var docs api.APIDocs
	if err := json.Unmarshal(data, &docs); err != nil {
		return fmt.Errorf("invalid JSON in %q: %w", specPath, err)
	}

	if docs.OpenAPI == "" {
		return fmt.Errorf("spec %q is missing required field: openapi", specPath)
	}

	if docs.Info == nil {
		return fmt.Errorf("spec %q is missing required field: info", specPath)
	}

	if docs.Info.Title == "" {
		return fmt.Errorf("spec %q is missing required field: info.title", specPath)
	}

	if docs.Info.Version == "" {
		return fmt.Errorf("spec %q is missing required field: info.version", specPath)
	}

	if docs.Paths == nil {
		return fmt.Errorf("spec %q is missing required field: paths", specPath)
	}

	if expectedVersion != "" {
		want := expectedVersion + ".json"
		if filepath.Base(specPath) != want {
			return fmt.Errorf("spec file %q does not match expected version filename %q", specPath, want)
		}
	}

	return nil
}
