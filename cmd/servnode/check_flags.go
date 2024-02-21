package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewCheckFlagCmd creates a command that checks the status of the json-rpc.allow-unprotected-txs flag
func NewCheckFlagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check-flags",
		Aliases: []string{"cf"},
		Short:   "Check the status of the json-rpc.allow-unprotected-txs flag",
		Run: func(cmd *cobra.Command, args []string) {
			flag := os.Getenv("JSON_RPC_ALLOW_UNPROTECTED_TXS")
			if flag == "" {
				fmt.Println("The flag json-rpc.allow-unprotected-txs is not set")
			} else {
				fmt.Printf("The flag json-rpc.allow-unprotected-txs is set to: %s\n", flag)
			}
		},
	}

	return cmd
}
