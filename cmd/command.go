package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/devsy-org/devsy-provider-terraform/pkg/terraform"
	"github.com/spf13/cobra"
)

// CommandCmd holds the cmd flags.
type CommandCmd struct{}

// NewCommandCmd defines a command.
func NewCommandCmd() *cobra.Command {
	cmd := &CommandCmd{}
	return &cobra.Command{
		Use:   "command",
		Short: "Command an instance",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			terraformProvider, err := terraform.NewProvider()
			if err != nil {
				return err
			}

			return cmd.Run(cobraCmd.Context(), terraformProvider)
		},
	}
}

// Run runs the command logic.
func (cmd *CommandCmd) Run(
	ctx context.Context,
	providerTerraform *terraform.TerraformProvider,
) error {
	command := os.Getenv("COMMAND")
	if command == "" {
		return fmt.Errorf("command environment variable is missing")
	}

	return terraform.Command(ctx, providerTerraform, command)
}
