// A blueprint that builds and publishes a Go CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dagger/go-cli/internal/dagger"
)

type GoCli struct {
	Source      *dagger.Directory
	GoVersion   string
	GithubToken *dagger.Secret
	GithubOwner string
	GithubRepo  string
}

func New(
	source *dagger.Directory,
	// Go version used to build the CLI
	goVersion string,
	// GitHub token used to create releases and upload assets
	githubToken *dagger.Secret,
	// GitHub repository owner
	githubOwner string,
	// GitHub repository name
	githubRepo string,
) *GoCli {
	return &GoCli{
		Source:      source,
		GoVersion:   goVersion,
		GithubToken: githubToken,
		GithubOwner: githubOwner,
		GithubRepo:  githubRepo,
	}
}

// Verify runs all repository and Go quality checks
func (m *GoCli) Verify(ctx context.Context) (string, error) {
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

// Build produces multi-platform binaries
func (m *GoCli) Build(
	// The binary name to build
	name string,
	// Go packages
	packages []string,
	// Platforms to build for (format: os/arch)
	// +optional
	// +default=["linux/amd64","linux/arm64"]
	platforms []string,
) *dagger.Directory {
	return dag.GoToolchain(m.GoVersion, m.Source).BuildMulti(name, packages, dagger.GoToolchainBuildMultiOpts{
		Platforms: platforms,
	})
}

// Release versions the project with semantic-release and uploads the binaries as assets
func (m *GoCli) Release(
	ctx context.Context,
	// The binary name to build
	name string,
	// Go packages
	packages []string,
	// Repository URL used by semantic-release
	// +optional
	repositoryUrl string,
	// +optional
	// +default=false
	dryRun bool,
) (string, error) {
	rt := dag.ReleaseToolchain(m.GithubToken)

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

	built := m.Build(name, packages, nil)
	paths, err := built.Glob(ctx, "bin/*/*/*")
	if err != nil {
		return "", err
	}

	if len(paths) == 0 {
		return "", fmt.Errorf("no binaries found in build output")
	}

	assets := dag.Directory()
	for _, p := range paths {
		parts := strings.Split(p, "/")
		assets = assets.WithFile(parts[len(parts)-1], built.File(p))
	}

	if _, err := rt.AttachAssets(ctx, version, m.GithubOwner, m.GithubRepo, assets); err != nil {
		return "", err
	}

	return version, nil
}
