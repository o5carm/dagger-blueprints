// A blueprint that builds and publishes a Go microservice.
package main

import (
	"context"
	"errors"
	"fmt"

	"dagger/go-service/internal/dagger"
)

type GoService struct {
	Source            *dagger.Directory
	GoVersion         string
	AlpineVersion     string
	RegistryAddress   string
	RegistryNamespace string
	RegistryUsername  string
	RegistrySecret    *dagger.Secret
	GithubToken       *dagger.Secret
	GithubOwner       string
	GithubRepo        string
}

func New(
	source *dagger.Directory,
	// Go version used to build the service
	goVersion string,
	// Alpine version used for the minimal rootfs
	alpineVersion string,
	// Registry address (e.g. "ghcr.io"). Empty disables publication.
	registryAddress string,
	// Registry namespace (e.g. "juli3nk")
	registryNamespace string,
	// Registry username
	registryUsername string,
	// Registry secret
	registrySecret *dagger.Secret,
	// GitHub token used to create releases and upload assets
	githubToken *dagger.Secret,
	// GitHub repository owner
	githubOwner string,
	// GitHub repository name
	githubRepo string,
) *GoService {
	return &GoService{
		Source:            source,
		GoVersion:         goVersion,
		AlpineVersion:     alpineVersion,
		RegistryAddress:   registryAddress,
		RegistryNamespace: registryNamespace,
		RegistryUsername:  registryUsername,
		RegistrySecret:    registrySecret,
		GithubToken:       githubToken,
		GithubOwner:       githubOwner,
		GithubRepo:        githubRepo,
	}
}

// Verify runs all repository and Go quality checks
func (m *GoService) Verify(ctx context.Context) (string, error) {
	var errs []error
	var report string

	repoOut, err := dag.RepositoryToolchain(m.Source).Validate(ctx)
	report += fmt.Sprintf("=== Repository ===\n%s\n", repoOut)
	if err != nil {
		errs = append(errs, fmt.Errorf("repository: %w", err))
	}

	goOut, err := dag.GoToolchain(m.GoVersion, m.Source).Validate(ctx)
	report += fmt.Sprintf("=== Go ===\n%s\n", goOut)
	if err != nil {
		errs = append(errs, fmt.Errorf("go: %w", err))
	}

	if len(errs) > 0 {
		return report, errors.Join(errs...)
	}

	return report, nil
}

// Build compiles the service and assembles a minimal OCI image
func (m *GoService) Build(
	// The binary name to build
	name string,
	// Go package to build
	mainPackage string,
	// Entrypoint of the image
	// +optional
	entrypoint []string,
	// Port to expose
	// +optional
	// +default=0
	port int,
	// Platform to build for
	// +optional
	// +default="linux/amd64"
	platform string,
) *dagger.Container {
	binary := dag.GoToolchain(m.GoVersion, m.Source).Build(name, []string{mainPackage}, dagger.GoToolchainBuildOpts{
		Static: true,
	})

	return dag.ContainerToolchain(m.AlpineVersion).Create(binary, "/usr/local/bin/"+name, dagger.ContainerToolchainCreateOpts{
		Entrypoint: entrypoint,
		Port:       port,
		Platform:   platform,
	})
}

// SBOM generates a Software Bill of Materials for an image
func (m *GoService) SBOM(
	image *dagger.Container,
	// Output format (spdx-json, cyclonedx-json, syft-json)
	// +optional
	// +default="spdx-json"
	format string,
) *dagger.File {
	return dag.SupplychainToolchain(image).Sbom(dagger.SupplychainToolchainSbomOpts{
		Format: format,
	})
}

// VulnerabilityReport generates a vulnerability report for an image
func (m *GoService) VulnerabilityReport(
	image *dagger.Container,
	// Trivy report format (table, json, sarif, cyclonedx, spdx)
	// +optional
	// +default="table"
	format string,
) *dagger.File {
	return dag.SupplychainToolchain(image).VulnerabilityReport(dagger.SupplychainToolchainVulnerabilityReportOpts{
		Format: format,
	})
}

// Scan runs a vulnerability scan on an image and returns its output
func (m *GoService) Scan(ctx context.Context, image *dagger.Container) (string, error) {
	return dag.SupplychainToolchain(image).Scan(ctx)
}

// Release versions the project with semantic-release and publishes image, SBOM and reports
func (m *GoService) Release(
	ctx context.Context,
	// The binary name to build
	name string,
	// Go package to build
	mainPackage string,
	// Repository URL used by semantic-release
	// +optional
	repositoryUrl string,
	// +optional
	// +default=false
	dryRun bool,
) (string, error) {
	rt := dag.ReleaseToolchain(name, m.RegistryAddress, m.RegistryNamespace, m.RegistryUsername, m.RegistrySecret, m.GithubToken)

	version, err := rt.Release(ctx, m.Source, dagger.ReleaseToolchainReleaseOpts{
		RepositoryURL: repositoryUrl,
		DryRun:        dryRun,
	})
	if err != nil {
		return "", err
	}

	if dryRun {
		return version, nil
	}

	image := m.Build(name, mainPackage, nil, 0, "")

	if _, err := rt.Publish(ctx, image, version, dagger.ReleaseToolchainPublishOpts{
		Latest: true,
	}); err != nil {
		return "", err
	}

	assets := dag.Directory().
		WithFile("sbom.spdx.json", m.SBOM(image, "spdx-json")).
		WithFile("vulnerability-report.txt", m.VulnerabilityReport(image, "table"))

	if _, err := rt.AttachAssets(ctx, version, m.GithubOwner, m.GithubRepo, assets); err != nil {
		return "", err
	}

	return version, nil
}
