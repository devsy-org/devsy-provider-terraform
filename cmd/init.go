package cmd

import (
	"context"
	"path/filepath"

	"github.com/devsy-org/devsy-provider-terraform/pkg/options"
	"github.com/devsy-org/devsy-provider-terraform/pkg/terraform"
	"github.com/devsy-org/devsy/pkg/config"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// InitCmd holds the cmd flags.
type InitCmd struct{}

// NewInitCmd defines a init.
func NewInitCmd() *cobra.Command {
	cmd := &InitCmd{}
	return &cobra.Command{
		Use:   "init",
		Short: "Init account",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return cmd.Run(cobraCmd.Context(), log.Default)
		},
	}
}

// Run runs the init logic.
func (cmd *InitCmd) Run(ctx context.Context, logs log.Logger) error {
	devsyPath, err := config.GetConfigDir()
	if err != nil {
		return err
	}

	terraformPath := filepath.Join(devsyPath, "bin", "terraform")

	project, err := options.FromEnvOrError(options.TERRAFORM_PROJECT)
	if err != nil {
		return err
	}

	// create provider
	provider := &terraform.TerraformProvider{
		Log:     logs,
		Bin:     terraformPath,
		Project: project,
	}

	return terraform.Install(ctx, provider)
}
