# push-to-rancher

Automates opening PRs against [rancher/rancher](https://github.com/rancher/rancher) when a new rancher-assets release is published.

## Workflows

### `.github/workflows/push-to-rancher.yml`

Standalone workflow that can be:
- **Manually triggered** via `workflow_dispatch` with any tag (including dev builds)
- **Called** from other workflows (e.g., `release.yml` for stable releases)

### `.github/workflows/release.yml`

Can be configured to automatically call `push-to-rancher.yml` for releases (after images are published).

## How it works

For each target Rancher branch:
1. Clone rancher/rancher
2. Update `defaultAssetsImage` in `build.yaml`
3. Run `go generate ./pkg/...` to update generated files
4. Commit changes
5. Push branch and create PR

## Target branches

Target branches are explicitly mapped to tag versions in `common.sh`:

```bash
declare -A VERSION_BRANCH_MAP=(
  ["2.16"]="main"
  ["2.15"]="release/v2.15"
)
```

**When to update:**
- **Release branch created**: Move main to next version, add release branch
  - Example: When `release/v2.16` is cut from main, update to:
    ```bash
    ["2.17"]="main"
    ["2.16"]="release/v2.16"
    ```
- **EOL version**: Remove the mapping
- **Multiple branches per version**: Use comma-separated list (no spaces)
  - Example: `["2.16"]="main,release/v2.16"`

**How it works:**
- Tag `v2.16-20260901T1200Z` → looks up `2.16` → updates branches: `main`
- Tag `v2.15-20260901T1200Z` → looks up `2.15` → updates branches: `release/v2.15`
- If version not in map → workflow fails with clear error message

### Filtering branches at runtime

Override which branches to process using `RANCHER_BRANCHES_OVERRIDE`:

**Local usage:**
```bash
# Only v2.16 and main
./run-local.sh --tag v2.16-20260901T1200Z --rancher-dir ~/rancher \
  --branches "release/v2.16,main"

# Single branch
./run-local.sh --tag v2.16-20260901T1200Z --rancher-dir ~/rancher \
  --branches "main"
```

**GHA usage:**
Set `RANCHER_BRANCHES_OVERRIDE` in the workflow file or as a workflow input:
```yaml
env:
  RANCHER_BRANCHES_OVERRIDE: "release/v2.16 main"
```

## Local usage

### Prerequisites

- **Bash**: All scripts require `bash` (not POSIX sh). They use bash-specific features like arrays, `[[`, and `BASH_SOURCE` for safety and clarity.
- **yq**: YAML processor for updating build.yaml
- **gh**: GitHub CLI for creating PRs
- **docker**: For validating images exist in registries

### Running locally

```bash
./.github/workflows/push-to-rancher/run-local.sh \
  --tag v2.16-20260901T1200Z \
  --rancher-dir /path/to/rancher \
  [--dry-run] \
  [--remote upstream] \
  [--branches "release/v2.16,main"]
```

Options:
- `--dry-run` - runs all local git work (commits to your rancher clone) but skips push and PR creation
- `--remote` - git remote name in your rancher clone (default: `origin`)
- `--branches` - comma-separated list of branches to process (overrides default list)

## Step sequence

| Script | What it does |
|---|---|
| `update-build-yaml.sh` | Updates `defaultAssetsImage` in `build.yaml` using `yq` |
| `create-prs.sh` | For each target branch: checkout, update, `go generate`, commit, push, create PR |
| `run-gha.sh` | GHA entry point: validates image exists, clones rancher/rancher, calls create-prs.sh |
| `run-local.sh` | Local entry point: parses args, sets up env, calls create-prs.sh |

## Key env vars

| Var | Description |
|---|---|
| `TAG` | rancher-assets tag (e.g. `v2.16-20260901T1200Z`) |
| `RANCHER_DIR` | Path to local rancher/rancher clone |
| `RANCHER_REMOTE` | Remote name in `RANCHER_DIR` (default: `origin`) |
| `RANCHER_BRANCHES_OVERRIDE` | Space or comma-separated list of branches to process (overrides default) |
| `DRY_RUN` | Set to `true` to skip push and PR creation |
| `SOURCE_REPO` | Source repo for PR body (default: `rancher/rancher-assets`) |

## GHA prerequisites

The workflow reads a GitHub App credential from Vault at:

```
secret/data/github/repo/rancher/rancher-assets/github/app-credentials
```

The app must have write access to `rancher/rancher` to push branches and open PRs.

## Error handling

The workflow continues processing branches even if one fails. Failed branches are logged at the end of the run. Common failure modes:

- Branch doesn't exist (e.g., when a new Rancher version isn't released yet)
- `go generate` fails (rare, usually indicates build.yaml format change)
- PR already exists for this tag on this branch
