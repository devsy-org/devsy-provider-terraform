package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/devsy-org/devsy-provider-terraform/pkg/terraform"
	"github.com/spf13/cobra"
)

// StatusCmd holds the cmd flags.
type StatusCmd struct{}

// NewStatusCmd defines a command.
func NewStatusCmd() *cobra.Command {
	cmd := &StatusCmd{}
	return &cobra.Command{
		Use:   "status",
		Short: "Status an instance",
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
func (cmd *StatusCmd) Run(
	ctx context.Context,
	providerTerraform *terraform.TerraformProvider,
) error {
	status, err := terraform.Status(ctx, providerTerraform)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(os.Stdout, status)
	return err
}
