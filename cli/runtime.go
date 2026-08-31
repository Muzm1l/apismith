package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"aegion-dynamic/apismith/environments"
	"aegion-dynamic/apismith/openapi"

	"github.com/joho/godotenv"
)

type runtime struct {
	cfg     *environments.Config
	catalog *openapi.Catalog
}

func loadDotEnv() {
	for _, p := range []string{".env", filepath.Join("config", ".env")} {
		_ = godotenv.Load(p)
	}
}

func loadRuntime() (*runtime, error) {
	loadDotEnv()
	path := configPath
	if path == "" {
		path = os.Getenv("CONSOLE_CONFIG")
	}
	if path == "" {
		path = "config/environments.yaml"
	}
	cfg, err := environments.Load(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	catalog, err := openapi.Load(cfg.OpenAPISpec)
	if err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	return &runtime{cfg: cfg, catalog: catalog}, nil
}
