package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	genericv1 "ocm.software/open-component-model/bindings/go/configuration/generic/v1/spec"
)

const (
	ocmConfigEnvKey     = "OCM_CONFIG"
	ocmConfigDirName    = ".ocm"
	ocmConfigFileName   = ocmConfigDirName + "/config"
	ocmNestedConfigName = ".ocmconfig"
)

// loadOCMConfig loads the OCM config from standard locations, mirroring the CLI's lookup order.
// Returns an empty config (not an error) when no config file is found.
func loadOCMConfig() (*genericv1.Config, error) {
	paths := ocmConfigPaths()
	if len(paths) == 0 {
		return &genericv1.Config{}, nil
	}

	cfgs := make([]*genericv1.Config, 0, len(paths))
	var errs []error
	for _, path := range paths {
		cfg, err := loadConfigFromPath(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading %s: %w", path, err))
			continue
		}
		cfgs = append(cfgs, cfg)
	}

	if len(cfgs) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return genericv1.FlatMap(cfgs...), nil
}

func loadConfigFromPath(path string) (_ *genericv1.Config, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	var cfg genericv1.Config
	if err := genericv1.Scheme.Decode(f, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ocmConfigPaths() []string {
	var paths []string
	for _, p := range []string{
		envConfigPath(),
		xdgOrHomePath(),
		workingDirPath(),
	} {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func envConfigPath() string {
	if p := os.Getenv(ocmConfigEnvKey); p != "" {
		if _, err := os.Stat(filepath.Clean(p)); err == nil {
			return p
		}
	}
	return ""
}

func xdgOrHomePath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		if p := checkConfigInDir(xdg); p != "" {
			return p
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if p := checkConfigInDir(filepath.Join(home, ".config")); p != "" {
			return p
		}
		if p := checkConfigInDir(home); p != "" {
			return p
		}
	}
	return ""
}

func workingDirPath() string {
	if wd, err := os.Getwd(); err == nil {
		return checkConfigInDir(wd)
	}
	return ""
}

func checkConfigInDir(base string) string {
	for _, name := range []string{ocmConfigFileName, ocmNestedConfigName} {
		p := filepath.Clean(filepath.Join(base, name))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
