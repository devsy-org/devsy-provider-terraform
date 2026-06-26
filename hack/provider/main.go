package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	providerName = "terraform"
	githubOwner  = "devsy-org"
	githubRepo   = "devsy-provider-terraform"
)

type Provider struct {
	Name         string            `yaml:"name"`
	Version      string            `yaml:"version"`
	Description  string            `yaml:"description"`
	Icon         string            `yaml:"icon"`
	OptionGroups []OptionGroup     `yaml:"optionGroups"`
	Options      Options           `yaml:"options"`
	Agent        Agent             `yaml:"agent"`
	Binaries     Binaries          `yaml:"binaries"`
	Exec         map[string]string `yaml:"exec"`
}

type OptionGroup struct {
	Name           string   `yaml:"name"`
	DefaultVisible bool     `yaml:"defaultVisible"`
	Options        []string `yaml:"options"`
}

type Options map[string]Option

type Option struct {
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Command     string `yaml:"command,omitempty"`
}

type Agent struct {
	Path                    string         `yaml:"path"`
	InactivityTimeout       string         `yaml:"inactivityTimeout"`
	InjectGitCredentials    string         `yaml:"injectGitCredentials"`
	InjectDockerCredentials string         `yaml:"injectDockerCredentials"`
	Exec                    map[string]any `yaml:"exec"`
}

type Binaries struct {
	TerraformProvider []Binary `yaml:"TERRAFORM_PROVIDER"`
}

type Binary struct {
	OS       string `yaml:"os"`
	Arch     string `yaml:"arch"`
	Path     string `yaml:"path"`
	Checksum string `yaml:"checksum"`
}

type buildConfig struct {
	version     string
	projectRoot string
	isRelease   bool
	checksums   map[string]string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("expected version as argument")
	}

	cfg, err := newBuildConfig(os.Args[1])
	if err != nil {
		return err
	}

	provider, err := buildProvider(cfg)
	if err != nil {
		return err
	}

	output, err := yaml.Marshal(provider)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	_, err = os.Stdout.Write(output)
	return err
}

func newBuildConfig(version string) (*buildConfig, error) {
	checksums, err := parseChecksums("./dist/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("parse checksums: %w", err)
	}

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		owner := getEnvOrDefault("GITHUB_OWNER", githubOwner)
		projectRoot = fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/%s",
			owner,
			githubRepo,
			version,
		)
	}

	isRelease := strings.Contains(projectRoot, "github.com") &&
		strings.Contains(projectRoot, "/releases/")

	return &buildConfig{
		version:     version,
		projectRoot: projectRoot,
		isRelease:   isRelease,
		checksums:   checksums,
	}, nil
}

func buildProvider(cfg *buildConfig) (Provider, error) {
	binaries, err := buildBinaries(cfg, allPlatforms())
	if err != nil {
		return Provider{}, err
	}
	return Provider{
		Name:         providerName,
		Version:      cfg.version,
		Description:  "Devsy on Terraform",
		Icon:         "https://raw.githubusercontent.com/devsy-org/devsy/main/desktop/src/renderer/public/icons/providers/terraform.svg",
		OptionGroups: buildOptionGroups(),
		Options:      buildOptions(),
		Agent:        buildAgent(),
		Binaries:     binaries,
		Exec: map[string]string{
			"init":    "${TERRAFORM_PROVIDER} init",
			"command": "${TERRAFORM_PROVIDER} command",
			"create":  "${TERRAFORM_PROVIDER} create",
			"delete":  "${TERRAFORM_PROVIDER} delete",
			"status":  "${TERRAFORM_PROVIDER} status",
		},
	}, nil
}

func buildOptionGroups() []OptionGroup {
	return []OptionGroup{
		{
			Name:           "Terraform options",
			DefaultVisible: true,
			Options: []string{
				"TERRAFORM_PROJECT",
				"REGION",
				"DISK_SIZE",
				"IMAGE_DISK",
				"INSTANCE_TYPE",
			},
		},
		{
			Name:           "Agent options",
			DefaultVisible: false,
			Options: []string{
				"AGENT_PATH",
				"INACTIVITY_TIMEOUT",
				"INJECT_DOCKER_CREDENTIALS",
				"INJECT_GIT_CREDENTIALS",
			},
		},
	}
}

func buildOptions() Options {
	return Options{
		"TERRAFORM_PROJECT": {
			Description: "The path or repo where the terraform files are. " +
				"E.g. ./examples/terraform or https://github.com/examples/terraform",
			Required: true,
		},
		"REGION": {
			Description: "The cloud region to create the VM in. E.g. us-west-1.",
			Required:    true,
		},
		"DISK_SIZE": {
			Description: "The disk size to use (GB).",
			Default:     "40",
		},
		"IMAGE_DISK": {
			Description: "The disk image to use.",
		},
		"INSTANCE_TYPE": {
			Description: "The machine type to use.",
		},
		"INACTIVITY_TIMEOUT": {
			Description: "If defined, will automatically stop the VM after the inactivity period.",
			Default:     "10m",
		},
		"INJECT_GIT_CREDENTIALS": {
			Description: "If Devsy should inject git credentials into the remote host.",
			Default:     "true",
		},
		"INJECT_DOCKER_CREDENTIALS": {
			Description: "If Devsy should inject docker credentials into the remote host.",
			Default:     "true",
		},
		"AGENT_PATH": {
			Description: "The path where to inject the Devsy agent to.",
			Default:     "/var/lib/toolbox/devsy",
		},
	}
}

//nolint:gosec // G101: template variables, not actual credentials
func buildAgent() Agent {
	return Agent{
		Path:                    "${AGENT_PATH}",
		InactivityTimeout:       "${INACTIVITY_TIMEOUT}",
		InjectGitCredentials:    "${INJECT_GIT_CREDENTIALS}",
		InjectDockerCredentials: "${INJECT_DOCKER_CREDENTIALS}",
		Exec: map[string]any{
			"shutdown": "shutdown -P now",
		},
	}
}

func buildBinaries(cfg *buildConfig, platforms []string) (Binaries, error) {
	list, err := buildBinaryList(cfg, platforms)
	if err != nil {
		return Binaries{}, err
	}
	return Binaries{TerraformProvider: list}, nil
}

func buildBinaryList(cfg *buildConfig, platforms []string) ([]Binary, error) {
	result := make([]Binary, 0, len(platforms))
	for _, platform := range platforms {
		binary, err := buildBinary(cfg, platform)
		if err != nil {
			return nil, err
		}
		result = append(result, binary)
	}
	return result, nil
}

func buildBinary(cfg *buildConfig, platform string) (Binary, error) {
	goOS, arch, ok := strings.Cut(platform, "/")
	if !ok {
		return Binary{}, fmt.Errorf("invalid platform %q", platform)
	}

	path, err := buildBinaryPath(cfg, platform, goOS, arch)
	if err != nil {
		return Binary{}, err
	}

	filename := buildFilename(goOS, arch)
	checksum, ok := cfg.checksums[filename]
	if !ok || checksum == "" {
		return Binary{}, fmt.Errorf("missing checksum for %s", filename)
	}

	return Binary{
		OS:       goOS,
		Arch:     arch,
		Path:     path,
		Checksum: checksum,
	}, nil
}

func buildBinaryPath(cfg *buildConfig, platform, goOS, arch string) (string, error) {
	dir := buildDir(platform)
	if dir == "" {
		return "", fmt.Errorf("unsupported platform %q", platform)
	}

	basePath, err := resolveBasePath(cfg, dir)
	if err != nil {
		return "", err
	}

	filename := buildFilename(goOS, arch)
	return joinPath(basePath, filename)
}

func resolveBasePath(cfg *buildConfig, dir string) (string, error) {
	if cfg.isRelease {
		return cfg.projectRoot, nil
	}

	if strings.HasPrefix(cfg.projectRoot, "http://") ||
		strings.HasPrefix(cfg.projectRoot, "https://") {
		return joinURLPath(cfg.projectRoot, dir)
	}

	absPath, err := filepath.Abs(cfg.projectRoot)
	if err != nil {
		return "", fmt.Errorf("abs PROJECT_ROOT: %w", err)
	}
	return filepath.Join(absPath, dir), nil
}

func joinPath(basePath, filename string) (string, error) {
	if strings.HasPrefix(basePath, "http://") || strings.HasPrefix(basePath, "https://") {
		return joinURLPath(basePath, filename)
	}
	return filepath.Join(basePath, filename), nil
}

func joinURLPath(base, elem string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	joined, err := url.JoinPath(parsed.String(), elem)
	if err != nil {
		return "", fmt.Errorf("join URL path: %w", err)
	}
	return joined, nil
}

func buildFilename(goOS, arch string) string {
	filename := fmt.Sprintf("devsy-provider-%s-%s-%s", providerName, goOS, arch)
	if goOS == "windows" {
		filename += ".exe"
	}
	return filename
}

func buildDir(platform string) string {
	dirs := map[string]string{
		"linux/amd64":   "build_linux_amd64_v1",
		"linux/arm64":   "build_linux_arm64_v8.0",
		"darwin/amd64":  "build_darwin_amd64_v1",
		"darwin/arm64":  "build_darwin_arm64_v8.0",
		"windows/amd64": "build_windows_amd64_v1",
	}
	return dirs[platform]
}

func allPlatforms() []string {
	return []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64"}
}

func parseChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path) //nolint:gosec // path is a build-time constant
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if checksum, filename, ok := strings.Cut(scanner.Text(), "  "); ok {
			checksums[strings.TrimSpace(filename)] = strings.TrimSpace(checksum)
		} else if checksum, filename, ok := strings.Cut(scanner.Text(), " "); ok {
			checksums[strings.TrimSpace(filename)] = strings.TrimSpace(checksum)
		}
	}

	return checksums, scanner.Err()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
