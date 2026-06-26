package terraform

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devsy-org/devsy-provider-terraform/pkg/options"
	"github.com/devsy-org/devsy/pkg/client"
	"github.com/devsy-org/devsy/pkg/config"
	"github.com/devsy-org/devsy/pkg/ssh"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-exec/tfexec"
	cp "github.com/otiai10/copy"
	"github.com/pkg/errors"
)

type TerraformProvider struct {
	Config     *options.Options
	Bin        string
	Project    string
	State      string
	WorkingDir string
}

func NewProvider() (*TerraformProvider, error) {
	providerConfig, err := options.FromEnv()
	if err != nil {
		return nil, err
	}

	devsyPath, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}

	terraformPath := filepath.Join(devsyPath, "bin", "terraform")

	project, err := options.FromEnvOrError(options.TERRAFORM_PROJECT)
	if err != nil {
		return nil, err
	}

	// create provider
	provider := &TerraformProvider{
		Config:     providerConfig,
		Bin:        terraformPath,
		Project:    project,
		State:      filepath.Join(providerConfig.MachineFolder, "main.tfstate"),
		WorkingDir: filepath.Join(providerConfig.MachineFolder, ".terraform"),
	}

	return provider, nil
}

func EnsureProject(providerTerraform *TerraformProvider) error {
	// if project is already in place, exit
	_, err := os.Stat(providerTerraform.WorkingDir)
	if err == nil {
		return nil
	}

	// if project is an url, try to clone it
	if strings.Contains(providerTerraform.Project, "http://") ||
		strings.Contains(providerTerraform.Project, "https://") {
		//nolint:gosec // G204: project repo URL comes from trusted provider config
		cmd := exec.Command(
			"git",
			"clone",
			providerTerraform.Project,
			providerTerraform.WorkingDir,
		)
		return cmd.Run()
	}

	// else we have a path, let's copy it to destination
	_, err = os.Stat(providerTerraform.Project)
	if err != nil {
		return errors.Errorf("terraform project not found")
	}

	return cp.Copy(providerTerraform.Project, providerTerraform.WorkingDir)
}

func Init(ctx context.Context, providerTerraform *TerraformProvider) (*tfexec.Terraform, error) {
	err := EnsureProject(providerTerraform)
	if err != nil {
		return nil, err
	}

	err = ensureBackend(providerTerraform)
	if err != nil {
		return nil, err
	}

	tf, err := tfexec.NewTerraform(providerTerraform.WorkingDir, providerTerraform.Bin)
	if err != nil {
		return nil, err
	}

	err = tf.Init(ctx, tfexec.Upgrade(true), tfexec.Reconfigure(true))
	if err != nil {
		return nil, err
	}

	return tf, nil
}

const backendOverrideFile = "devsy_backend_override.tf"

func ensureBackend(providerTerraform *TerraformProvider) error {
	overridePath := filepath.Join(providerTerraform.WorkingDir, backendOverrideFile)

	hasUserBackend, err := projectHasBackend(providerTerraform.WorkingDir)
	if err != nil {
		return err
	}

	if hasUserBackend {
		if err := os.Remove(overridePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	statePath := filepath.ToSlash(providerTerraform.State)
	content := fmt.Sprintf(`terraform {
  backend "local" {
    path = %q
  }
}
`, statePath)

	return os.WriteFile(overridePath, []byte(content), 0o600)
}

func projectHasBackend(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == backendOverrideFile {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}

		found, err := fileDeclaresBackend(filepath.Join(dir, entry.Name()))
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	return false, nil
}

func fileDeclaresBackend(path string) (bool, error) {
	content, err := os.ReadFile(path) //nolint:gosec // G304: path is within the trusted working dir
	if err != nil {
		return false, err
	}

	file, diags := hclsyntax.ParseConfig(content, filepath.Base(path), hcl.InitialPos)
	if diags.HasErrors() {
		return false, nil
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return false, nil
	}

	return bodyDeclaresBackend(body), nil
}

func bodyDeclaresBackend(body *hclsyntax.Body) bool {
	for _, block := range body.Blocks {
		if block.Type != "terraform" {
			continue
		}
		for _, inner := range block.Body.Blocks {
			if inner.Type == "backend" || inner.Type == "cloud" {
				return true
			}
		}
	}

	return false
}

func Install(ctx context.Context, providerTerraform *TerraformProvider) error {
	//nolint:gosec // G204: terraform binary path is derived from trusted config dir
	err := exec.Command(providerTerraform.Bin).Run()
	if err == nil {
		return nil
	}

	destPath := filepath.Dir(providerTerraform.Bin)

	err = os.MkdirAll(destPath, 0o750)
	if err != nil {
		return err
	}

	installer := &releases.ExactVersion{
		InstallDir: destPath,
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion("1.4.0")),
	}

	_, err = installer.Install(ctx)
	return err
}

func Delete(ctx context.Context, providerTerraform *TerraformProvider) error {
	tf, err := Init(ctx, providerTerraform)
	if err != nil {
		return err
	}

	return tf.Destroy(
		ctx,
		tfexec.Lock(false),
		tfexec.Refresh(true),
		tfexec.Parallelism(99),
	)
}

func Command(ctx context.Context, providerTerraform *TerraformProvider, command string) error {
	// get private key
	privateKey, err := ssh.GetPrivateKeyRawBase(providerTerraform.Config.MachineFolder)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	// get external address
	externalIP, err := getExternalIP(ctx, providerTerraform)
	if err != nil || externalIP == "" {
		return fmt.Errorf(
			"instance %s doesn't have an external nat ip",
			providerTerraform.Config.MachineID,
		)
	}

	sshClient, err := ssh.NewSSHClient("devsy", externalIP+":22", privateKey)
	if err != nil {
		return errors.Wrap(err, "create ssh client")
	}
	defer func() { _ = sshClient.Close() }()

	// run command
	return ssh.Run(ctx, ssh.RunOptions{
		Client:  sshClient,
		Command: command,
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})
}

func Create(ctx context.Context, providerTerraform *TerraformProvider) error {
	tf, err := Init(ctx, providerTerraform)
	if err != nil {
		return err
	}

	publicKey, err := publicKey(providerTerraform)
	if err != nil {
		return err
	}

	vars := terraformVars(providerTerraform, publicKey)

	applyOpts := append([]tfexec.ApplyOption{
		tfexec.Lock(false),
		tfexec.Refresh(true),
		tfexec.Parallelism(99),
	}, varsToApplyOptions(vars)...)

	err = tf.Apply(ctx, applyOpts...)
	if err != nil {
		return err
	}

	refreshOpts := append([]tfexec.RefreshCmdOption{
		tfexec.Lock(false),
	}, varsToRefreshOptions(vars)...)

	return tf.Refresh(ctx, refreshOpts...)
}

func getExternalIP(ctx context.Context, providerTerraform *TerraformProvider) (string, error) {
	tf, err := Init(ctx, providerTerraform)
	if err != nil {
		return "", err
	}

	output, err := tf.Output(ctx)
	if err != nil {
		return "", err
	}

	if output["public_ip"].Value == nil {
		return "", errors.Errorf("output not found")
	}

	return strings.ReplaceAll(string(output["public_ip"].Value), "\"", ""), nil
}

func Status(ctx context.Context, providerTerraform *TerraformProvider) (client.Status, error) {
	tf, err := Init(ctx, providerTerraform)
	if err != nil {
		return client.StatusNotFound, err
	}

	publicKey, err := publicKey(providerTerraform)
	if err != nil {
		return client.StatusNotFound, err
	}

	refreshOpts := append([]tfexec.RefreshCmdOption{
		tfexec.Lock(false),
	}, varsToRefreshOptions(terraformVars(providerTerraform, publicKey))...)

	err = tf.Refresh(ctx, refreshOpts...)
	if err != nil {
		return client.StatusNotFound, err
	}

	state, err := tf.Show(ctx)
	if err != nil {
		return client.StatusNotFound, err
	}

	if state.Values == nil {
		return client.StatusNotFound, nil
	}
	if state.Values.Outputs != nil {
		return client.StatusRunning, nil
	}

	return client.StatusBusy, nil
}

// publicKey loads and decodes the machine's public SSH key.
func publicKey(providerTerraform *TerraformProvider) (string, error) {
	publicKeyBase, err := ssh.GetPublicKeyBase(providerTerraform.Config.MachineFolder)
	if err != nil {
		return "", err
	}

	publicKey, err := base64.StdEncoding.DecodeString(publicKeyBase)
	if err != nil {
		return "", err
	}

	return string(publicKey), nil
}

// terraformVars builds the set of input variables passed to the terraform project.
func terraformVars(providerTerraform *TerraformProvider, publicKey string) []string {
	return []string{
		"disk_image=" + providerTerraform.Config.DiskImage,
		"disk_size=" + providerTerraform.Config.DiskSizeGB,
		"instance_type=" + providerTerraform.Config.MachineType,
		"machine_name=" + providerTerraform.Config.MachineID,
		"region=" + providerTerraform.Config.Zone,
		"ssh_key=" + publicKey,
	}
}

func varsToApplyOptions(vars []string) []tfexec.ApplyOption {
	opts := make([]tfexec.ApplyOption, 0, len(vars))
	for _, v := range vars {
		opts = append(opts, tfexec.Var(v))
	}
	return opts
}

func varsToRefreshOptions(vars []string) []tfexec.RefreshCmdOption {
	opts := make([]tfexec.RefreshCmdOption, 0, len(vars))
	for _, v := range vars {
		opts = append(opts, tfexec.Var(v))
	}
	return opts
}
