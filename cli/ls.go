package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"aegion-dynamic/apismith/openapi"

	"github.com/spf13/cobra"
)

func newLSCmd() *cobra.Command {
	var tag, search string
	cmd := &cobra.Command{
		Use:     "ls",
		Short:   "List operations from the OpenAPI spec",
		Long:    lsHelp,
		Example: lsExamples,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := loadRuntime()
			if err != nil {
				return err
			}
			eps := rt.catalog.Filter(tag, search)
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(eps)
			}
			printEndpointList(eps)
			return nil
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "filter by OpenAPI tag")
	cmd.Flags().StringVarP(&search, "search", "s", "", "search method, path, operationId, or summary")
	return cmd
}

func printEndpointList(eps []openapi.Endpoint) {
	for _, ep := range eps {
		summary := strings.TrimSpace(ep.Summary)
		op := ep.OperationID
		if op != "" {
			op = "  " + op
		}
		fmt.Printf("%s %s  %s%s\n", colorMethod(ep.Method), ep.Path, summary, dim(op))
	}
}
