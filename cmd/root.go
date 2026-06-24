package cmd

import (
	"errors"
	"os"
	"os/exec"

	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

// NewRootCmd returns a new root command.
func NewRootCmd() *cobra.Command {
	terraformCmd := &cobra.Command{
		Use:           "devsy-provider-terraform",
		Short:         "terraform Provider commands",
		SilenceErrors: true,
		SilenceUsage:  true,

		PersistentPreRunE: func(cobraCmd *cobra.Command, args []string) error {
			log.Default.MakeRaw()
			return nil
		},
	}

	return terraformCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// build the root command
	rootCmd := BuildRoot()

	// execute command
	err := rootCmd.Execute()
	if err != nil {
		var sshExitErr *ssh.ExitError
		if errors.As(err, &sshExitErr) {
			os.Exit(sshExitErr.ExitStatus())
		}
		var execExitErr *exec.ExitError
		if errors.As(err, &execExitErr) {
			if len(execExitErr.Stderr) > 0 {
				log.Default.ErrorStreamOnly().Error(string(execExitErr.Stderr))
			}
			os.Exit(execExitErr.ExitCode())
		}

		log.Default.Fatal(err)
	}
}

// BuildRoot creates a new root command from the available subcommands.
func BuildRoot() *cobra.Command {
	rootCmd := NewRootCmd()

	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewCreateCmd())
	rootCmd.AddCommand(NewDeleteCmd())
	rootCmd.AddCommand(NewCommandCmd())
	rootCmd.AddCommand(NewStatusCmd())
	return rootCmd
}
