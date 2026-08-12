// A blueprint that validates and releases a Go library.
package main

import (
	"context"
	"errors"
	"fmt"

	"dagger/go-library/internal/dagger"
)

type GoLibrary struct {
	Source      *dagger.Directory
	GoVersion   string
	GithubToken *dagger.Secret
}

func New(
	source *dagger.Directory,
	// Go version used to validate the library
	goVersion string,
	// GitHub token used to create releases
	githubToken *dagger.Secret,
) *GoLibrary {
	return &GoLibrary{
		Source:      source,
		GoVersion:   goVersion,
		GithubToken: githubToken,
	}
}

// Verify runs all repository and Go quality checks
func (m *GoLibrary) Verify(ctx context.Context) (string, error) {
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

// GenerateDocs generates API documentation
func (m *GoLibrary) GenerateDocs() *dagger.Directory {
	return dag.GoToolchain(m.GoVersion, m.Source).GenerateDocs()
}

// Release versions the project with semantic-release
func (m *GoLibrary) Release(
	ctx context.Context,
	// Repository URL used by semantic-release
	// +optional
	repositoryUrl string,
	// +optional
	// +default=false
	dryRun bool,
) (string, error) {
	return dag.ReleaseToolchain(m.GithubToken).Release(ctx, m.Source, dagger.ReleaseToolchainReleaseOpts{
		RepositoryURL: repositoryUrl,
		DryRun:        dryRun,
	})
}
