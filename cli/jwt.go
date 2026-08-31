package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"aegion-dynamic/apismith/auth/cognito"

	"github.com/spf13/cobra"
)

func newJWTCmd() *cobra.Command {
	var (
		envID     string
		tokenOnly bool
		clientID  string
		username  string
		password  string
	)
	cmd := &cobra.Command{
		Use:     "jwt [credentials-file]",
		Short:   "Generate a Cognito access token",
		Long:    jwtHelp,
		Example: jwtExamples,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loadDotEnv()
			var cfg cognito.Config
			var err error
			if len(args) == 1 {
				loaded, loadErr := cognito.LoadFile(args[0])
				if loadErr != nil {
					return loadErr
				}
				cfg = *loaded
			} else {
				rt, rtErr := loadRuntime()
				if rtErr != nil {
					return rtErr
				}
				if envID == "" {
					envID = rt.cfg.DefaultEnvironment
				}
				cfg, err = rt.cfg.CognitoConfigFor(envID, clientID, username, password)
				if err != nil {
					return err
				}
			}
			if clientID != "" {
				cfg.ClientID = clientID
			}
			if username != "" {
				cfg.Username = username
			}
			if password != "" {
				cfg.Password = password
			}
			tokens, err := cognito.Generate(context.Background(), cfg)
			if err != nil {
				return err
			}
			return printTokens(tokens, tokenOnly)
		},
	}
	cmd.Flags().StringVarP(&envID, "env", "e", "", "environment id (default from config)")
	cmd.Flags().BoolVar(&tokenOnly, "token-only", false, "print the access token only (pipeable)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "override Cognito client id")
	cmd.Flags().StringVar(&username, "username", "", "override username")
	cmd.Flags().StringVar(&password, "password", "", "override password")
	return cmd
}

func printTokens(tokens *cognito.Tokens, tokenOnly bool) error {
	if tokenOnly {
		fmt.Println(tokens.AccessToken)
		return nil
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"access_token": tokens.AccessToken,
			"id_token":     tokens.IDToken,
			"token_type":   tokens.TokenType,
			"expires_in":   tokens.ExpiresIn,
		})
	}
	fmt.Println("Access Token")
	fmt.Println(tokens.AccessToken)
	fmt.Println()
	if tokens.IDToken != "" {
		fmt.Println("ID Token")
		fmt.Println(tokens.IDToken)
		fmt.Println()
	}
	fmt.Printf("token_type  %s\n", tokens.TokenType)
	fmt.Printf("expires_in  %ds\n", tokens.ExpiresIn)
	return nil
}
