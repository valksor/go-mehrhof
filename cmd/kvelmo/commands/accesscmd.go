package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var (
	accessTokenRole       string
	accessTokenLabel      string
	accessTokenListJSON   bool
	accessTokenCreateJSON bool
)

var AccessCmd = &cobra.Command{
	Use:   "access",
	Short: "Socket access token management",
	Long:  "Create, revoke, and list access tokens for optional socket authentication.",
}

var accessTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage access tokens",
}

var accessTokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new access token",
	RunE:  runAccessTokenCreate,
}

var accessTokenRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an access token",
	Args:  cobra.ExactArgs(1),
	RunE:  runAccessTokenRevoke,
}

var accessTokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "List access tokens",
	RunE:  runAccessTokenList,
}

func init() {
	AccessCmd.AddCommand(accessTokenCmd)
	accessTokenCmd.AddCommand(accessTokenCreateCmd)
	accessTokenCmd.AddCommand(accessTokenRevokeCmd)
	accessTokenCmd.AddCommand(accessTokenListCmd)

	accessTokenCreateCmd.Flags().StringVar(&accessTokenRole, "role", "operator", "Token role (operator or viewer)")
	accessTokenCreateCmd.Flags().StringVar(&accessTokenLabel, "label", "", "Human-readable label for the token")
	accessTokenCreateCmd.Flags().BoolVar(&accessTokenCreateJSON, "json", false, "Output raw JSON response")
	accessTokenListCmd.Flags().BoolVar(&accessTokenListJSON, "json", false, "Output raw JSON response")
}

func connectGlobalSocket(timeout time.Duration) (*socket.Client, error) {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return nil, fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(timeout))
	if err != nil {
		return nil, fmt.Errorf("connect to global socket: %w", err)
	}

	return client, nil
}

func runAccessTokenCreate(_ *cobra.Command, _ []string) error {
	client, err := connectGlobalSocket(5 * time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "access.token.create", map[string]any{
		"role":  accessTokenRole,
		"label": accessTokenLabel,
	})
	if err != nil {
		return fmt.Errorf("access.token.create: %w", err)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if accessTokenCreateJSON {
		out, jsonErr := json.MarshalIndent(map[string]string{"token": result.Token}, "", "  ")
		if jsonErr != nil {
			return fmt.Errorf("marshal JSON: %w", jsonErr)
		}
		fmt.Println(string(out))

		return nil
	}

	fmt.Printf("Token created: %s\n", result.Token)
	fmt.Println("Store this token securely — it cannot be retrieved later.")

	return nil
}

func runAccessTokenRevoke(_ *cobra.Command, args []string) error {
	client, err := connectGlobalSocket(5 * time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "access.token.revoke", map[string]any{
		"id": args[0],
	})
	if err != nil {
		return fmt.Errorf("access.token.revoke: %w", err)
	}

	var result struct {
		Revoked bool `json:"revoked"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Println("Token revoked.")

	return nil
}

func runAccessTokenList(_ *cobra.Command, _ []string) error {
	client, err := connectGlobalSocket(5 * time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "access.token.list", nil)
	if err != nil {
		return fmt.Errorf("access.token.list: %w", err)
	}

	var result struct {
		Tokens json.RawMessage `json:"tokens"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if accessTokenListJSON {
		out, jsonErr := json.MarshalIndent(result.Tokens, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(result.Tokens))

			return nil
		}
		fmt.Println(string(out))

		return nil
	}

	// Check if tokens array is empty
	var tokens []any
	if err := json.Unmarshal(result.Tokens, &tokens); err != nil {
		return fmt.Errorf("parse tokens: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Println("No access tokens configured.")

		return nil
	}

	data, err := json.MarshalIndent(result.Tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("format: %w", err)
	}

	fmt.Println(string(data))

	return nil
}
