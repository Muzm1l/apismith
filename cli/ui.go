package cli

import (
	"fmt"
	"net/http"

	"aegion-dynamic/apismith/environments"
	"aegion-dynamic/apismith/server"

	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ui",
		Short:   "Start the local API explorer",
		Long:    uiHelp,
		Example: uiExamples,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunUI()
		},
	}
}

// RunUI starts the local explorer process.
func RunUI() error {
	rt, err := loadRuntime()
	if err != nil {
		return err
	}
	addr := rt.cfg.Listen
	if addr == "" {
		addr = environments.DefaultListen
	}
	fmt.Printf("apismith ui\n")
	fmt.Printf("  spec        %s\n", rt.cfg.OpenAPISpec)
	fmt.Printf("  endpoints   %d\n", len(rt.catalog.Endpoints))
	fmt.Printf("  environment %s\n", rt.cfg.DefaultEnvironment)
	fmt.Printf("  listen      http://localhost%s\n\n", addr)
	return http.ListenAndServe(addr, server.New(rt.cfg, rt.catalog).Handler())
}
