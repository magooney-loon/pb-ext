package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Note: Uses env vars defined in openapi_embedded_loader.go:
// - specsDirEnv = "PB_EXT_OPENAPI_SPECS_DIR"
// - disableSpecsEnv = "PB_EXT_DISABLE_OPENAPI_SPECS"

type VersionConfigProvider func() map[string]*APIDocsConfig
type VersionManagerInitializer func() (*APIVersionManager, error)
type RouteRegistrar func(vm *APIVersionManager) error

type SpecGenerator struct {
	versionConfigs VersionConfigProvider
	routeRegistrar RouteRegistrar
	vmInitializer  VersionManagerInitializer
}

func NewSpecGenerator(configs VersionConfigProvider, routes RouteRegistrar) *SpecGenerator {
	return &SpecGenerator{
		versionConfigs: configs,
		routeRegistrar: routes,
	}
}

func NewSpecGeneratorWithInitializer(initializer VersionManagerInitializer) *SpecGenerator {
	return &SpecGenerator{
		vmInitializer: initializer,
	}
}

func (sg *SpecGenerator) Generate(outputDir string, onlyVersion string) error {
	if outputDir == "" {
		return fmt.Errorf("spec output directory is required")
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create specs directory %q: %w", outputDir, err)
	}

	originalspecsDirEnv, hadspecsDirEnv := os.LookupEnv(specsDirEnv)
	if err := os.Unsetenv(specsDirEnv); err != nil {
		return fmt.Errorf("failed to unset %s for generation: %w", specsDirEnv, err)
	}
	originalDisableEmbeddedEnv, hadDisableEmbeddedEnv := os.LookupEnv(disableSpecsEnv)
	if err := os.Setenv(disableSpecsEnv, "1"); err != nil {
		return fmt.Errorf("failed to set %s for generation: %w", disableSpecsEnv, err)
	}
	defer func() {
		if hadspecsDirEnv {
			_ = os.Setenv(specsDirEnv, originalspecsDirEnv)
		} else {
			_ = os.Unsetenv(specsDirEnv)
		}

		if hadDisableEmbeddedEnv {
			_ = os.Setenv(disableSpecsEnv, originalDisableEmbeddedEnv)
		} else {
			_ = os.Unsetenv(disableSpecsEnv)
		}
	}()

	var vm *APIVersionManager
	var err error

	if sg.vmInitializer != nil {
		vm, err = sg.vmInitializer()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}
		if err := vm.RegisterAllVersionRoutesForDocs(); err != nil {
			return fmt.Errorf("failed to register routes for docs generation: %w", err)
		}
	} else {
		vm = InitializeVersionedSystem(sg.versionConfigs(), "v1")
		if sg.routeRegistrar != nil {
			if err := sg.routeRegistrar(vm); err != nil {
				return fmt.Errorf("failed to register routes for docs generation: %w", err)
			}
		}
	}

	versions := vm.GetAllVersions()
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
		registry, err := vm.GetVersionRegistry(version)
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

	return ValidateSpecs(outputDir, versions)
}

func (sg *SpecGenerator) Validate(specsDir string) error {
	if specsDir == "" {
		return fmt.Errorf("specs directory is required")
	}

	var versionList []string

	if sg.vmInitializer != nil {
		vm, err := sg.vmInitializer()
		if err != nil {
			return fmt.Errorf("failed to initialize version manager: %w", err)
		}
		versionList = vm.GetAllVersions()
	} else {
		versions := sg.versionConfigs()
		versionList = make([]string, 0, len(versions))
		for v := range versions {
			versionList = append(versionList, v)
		}
	}
	sort.Strings(versionList)

	return ValidateSpecs(specsDir, versionList)
}

func ValidateSpecs(specsDir string, versions []string) error {
	if len(versions) == 0 {
		return fmt.Errorf("no configured API versions found")
	}

	for _, version := range versions {
		specPath := filepath.Join(specsDir, version+".json")
		if err := ValidateSpecFile(specPath, version); err != nil {
			return err
		}
	}

	return nil
}

func ValidateSpecFile(specPath string, expectedVersion string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("missing or unreadable spec file %q: %w", specPath, err)
	}

	var docs APIDocs
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
