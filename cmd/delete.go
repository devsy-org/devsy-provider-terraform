package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-terraform/pkg/terraform"
	"github.com/spf13/cobra"
)

// DeleteCmd holds the cmd flags.
type DeleteCmd struct{}

// NewDeleteCmd defines a command.
func NewDeleteCmd() *cobra.Command {
	cmd := &DeleteCmd{}
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete an instance",
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
func (cmd *DeleteCmd) Run(
	ctx context.Context,
	providerTerraform *terraform.TerraformProvider,
) error {
	return terraform.Delete(ctx, providerTerraform)
}
