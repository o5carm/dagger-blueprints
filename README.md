## Blueprints

Each blueprint takes the repository and infra configuration in its constructor, then exposes `Verify()` and the build/release pipeline.

### go-service

Builds and publishes a Go microservice: `Verify` → `Build` (minimal image) → `Release` (image + SBOM + vulnerability report + GitHub release).

Constructor: `New(source *Directory, goVersion, alpineVersion string, githubToken *Secret, githubOwner, githubRepo string)`.

Constructor flags:

```bash
COMMON="--source=./repo --go-version=1.22 --alpine-version=3.20 \
--github-token=env:GH_TOKEN --github-owner=owner --github-repo=repo"
```

Functions:

```bash
# All repository + Go checks
dagger call $COMMON verify

# Build the image (static binary in a minimal container)
dagger call $COMMON build \
  --name=hello --main-package=. --entrypoint=/usr/local/bin/hello --port=8080

# Supply chain artifacts (on any image)
dagger call $COMMON sbom --image=alpine:3.20 --format=spdx-json
dagger call $COMMON vulnerability-report --image=alpine:3.20 --format=json
dagger call $COMMON scan --image=alpine:3.20

# Version + publish image + attach SBOM/vuln report
dagger call $COMMON release \
  --name=hello --main-package=. \
  --registry-address=ghcr.io --registry-namespace=owner \
  --registry-username=owner --registry-secret=env:REG_TOKEN \
  --repository-url=https://github.com/owner/repo --dry-run
```

`build(name, mainPackage, entrypoint?=null, port?=0, platform?="linux/amd64")` → `*Container`.
`sbom(image *Container, format?="spdx-json")`, `vulnerability-report(image *Container, format?="table")` → `*File`.
`scan(ctx, image *Container)` → `string`.
`release(ctx, name, mainPackage string, repositoryUrl?="", dryRun?=false, registryAddress?="", registryNamespace?="", registryUsername?="", registrySecret?=*Secret)` → `string`. With `dryRun=true`, semantic-release runs but **publish and asset upload are skipped**. Registry params are optional and only used when not in dry-run.

### go-library

Validates and releases a Go library: `Verify` → `GenerateDocs` → `Release`.

Constructor: `New(source *Directory, goVersion string, githubToken *Secret)`.

```bash
dagger call --source=./repo --go-version=1.22 --github-token=env:GH_TOKEN verify
dagger call --source=./repo --go-version=1.22 --github-token=env:GH_TOKEN generate-docs
dagger call --source=./repo --go-version=1.22 --github-token=env:GH_TOKEN release \
  --repository-url=https://github.com/owner/repo --dry-run
```

Functions:
- `verify(ctx)` → `string`.
- `generate-docs()` → `*Directory`.
- `release(ctx, repositoryUrl?="", dryRun?=false)` → `string`.

### go-cli

Builds and releases a Go CLI: `Verify` → `Build` (multi-platform binaries) → `Release` (GitHub release + binaries as assets).

Constructor: `New(source *Directory, goVersion string, githubToken *Secret, githubOwner, githubRepo string)`.

```bash
dagger call --source=./repo --go-version=1.22 \
  --github-token=env:GH_TOKEN --github-owner=owner --github-repo=repo verify

dagger call --source=./repo --go-version=1.22 \
  --github-token=env:GH_TOKEN --github-owner=owner --github-repo=repo build \
  --name=hello --packages=. --platforms=linux/amd64 --platforms=linux/arm64

dagger call --source=./repo --go-version=1.22 \
  --github-token=env:GH_TOKEN --github-owner=owner --github-repo=repo release \
  --name=hello --packages=. --repository-url=https://github.com/owner/repo --dry-run
```

Functions:
- `verify(ctx)` → `string`.
- `build(name string, packages []string, platforms?=["linux/amd64","linux/arm64"])` → `*Directory`.
- `release(ctx, name string, packages []string, repositoryUrl?="", dryRun?=false)` → `string`. With `dryRun=true`, semantic-release runs but asset upload is skipped.
