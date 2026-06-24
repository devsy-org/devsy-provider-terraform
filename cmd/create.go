package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-terraform/pkg/terraform"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// CreateCmd holds the cmd flags.
type CreateCmd struct{}

// NewCreateCmd defines a command.
func NewCreateCmd() *cobra.Command {
	cmd := &CreateCmd{}
	return &cobra.Command{
		Use:   "create",
		Short: "Create an instance",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			terraformProvider, err := terraform.NewProvider(log.Default)
			if err != nil {
				return err
			}

			return cmd.Run(cobraCmd.Context(), terraformProvider)
		},
	}
}

// Run runs the command logic.
func (cmd *CreateCmd) Run(
	ctx context.Context,
	providerTerraform *terraform.TerraformProvider,
) error {
	return terraform.Create(ctx, providerTerraform)
}
