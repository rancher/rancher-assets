# push-to-rancher

Automates opening PRs against [rancher/rancher](https://github.com/rancher/rancher) when a new rancher-assets release is published.

## Workflows

### `.github/workflows/push-to-rancher.yml`

Standalone workflow that can be:
- **Automatically triggered** when a release is published (uses the release tag and commit)
- **Manually triggered** via `workflow_dispatch` with a git ref and rancher minor version
- **Called** from other workflows via `workflow_call` with a git ref and rancher minor version

**Trigger modes:**

1. **Release published** (automatic):
   - Uses the release tag (e.g., `v2.16-20260901T1200Z`) and commit SHA from the release event
   - Extracts rancher minor version from the tag
   
2. **Manual/workflow_call**:
   - Requires `git_ref` (commit SHA or tag) and `rancher_minor` (e.g., `2.16`)
   - Resolves the image tag using `resolve-image-tag.sh` (finds tags matching the version pattern at that commit)
   - Prefers non-dev tags if multiple tags exist

### `.github/workflows/release.yml`

Can be configured to automatically call `push-to-rancher.yml` for releases (after images are published).

## How it works

For each target Rancher branch:
1. Clone rancher/rancher
2. Update `defaultAssetsImage` in `build.yaml`
3. Run `go generate ./pkg/...` to update generated files
4. Commit changes
5. For a dev image of a branch on the staging catalog, update the chart pins and commit them as a second commit (see [Chart pins](#chart-pins))
6. Push branch and create PR

## Chart pins

`update-pins.sh` sets the pins of rancher/rancher to the newest staged version of each chart for the Rancher line of the image, in a second commit of the same PR, so that the pins do not have to be changed by hand after the image.

**When it applies.** Only when both are true: the tag ends in `-dev`, and the `build.yaml` of the target branch has `chartDefaultBranch: staging`. Anything else (a production image, a line still on the old catalog system) exits with 3 and the PR has the image commit only.

**Which version.** The charts commit inside the image is read from `lock.yaml` of the tag (`chart-versions.<minor>.upstream-refs.dev.charts.commit`), and its `index.yaml` from `rancher/charts` at that commit. For each pin, the version is the **first entry of the chart whose major is the one of the line**. The index is ordered by cargueiro, newest first, across every Rancher line, so the first entry of a chart can be a version of the next minor, and the major is what tells them apart. Chart versions are `1XY.Z.W+upA.B.C` and `1XY` follows the Rancher minor: 2.10 is 105, 2.16 is 111, 2.17 is 112 (`versions.majors` in `rules.yaml` of rancher/cargueiro), so the script uses `95 + minor`. Semver is not used: it ignores everything after the `+`, so two release candidates of the same upstream version compare equal.

**What it sets.**

| Pin | Chart | Where |
|---|---|---|
| `fleetVersion` | `fleet` | `build.yaml` |
| `remoteDialerProxyVersion` | `remotedialer-proxy` | `build.yaml` |
| `turtlesVersion` | `rancher-turtles` | `build.yaml` |
| `webhookVersion` | `rancher-webhook` | `build.yaml` |
| `aksOperatorVersion`, `eksOperatorVersion`, `gkeOperatorVersion` | `rancher-aks-operator`, `rancher-eks-operator`, `rancher-gke-operator` | `build.yaml`, if the branch has the key |
| `clusterAPIControllerImage` | the image with `dependency: cluster-api` in the `helm.sh/images` of the turtles entry chosen above | `build.yaml` |
| `CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION` | `system-upgrade-controller` | `package/Dockerfile` |

A key that the branch does not have is not added. A pin whose chart has no entry for the major of the line stays as it is, and the PR body says so.

**It never blocks the image.** If the pins step fails (the index cannot be read, `go generate` fails, the commit fails) its changes are dropped, a warning goes to the summary, and the PR has the image commit only. If a check of rancher/rancher fails because of a chart, the second commit can be dropped from the branch and the image merged alone.

**Tests.** `tests/update-pins_test.sh` runs the script against fixtures, with no network and no clone; `scripts/test` runs it in CI.

## Target branches

Target branches are explicitly mapped to tag versions in `common.sh`:

```bash
declare -A VERSION_BRANCH_MAP=(
  ["2.16"]="main,release/v2.16"
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
- Tag `v2.16-20260901T1200Z` → looks up `2.16` → updates branches: `main` and `release/v2.16`
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
| `resolve-image-tag.sh` | (GHA only) Resolves image tag from git ref + rancher minor for manual triggers |
| `update-build-yaml.sh` | Updates `defaultAssetsImage` in `build.yaml` using `yq` |
| `update-pins.sh` | Sets the chart pins of a dev image of a branch on the staging catalog (see [Chart pins](#chart-pins)) |
| `create-prs.sh` | For each target branch: checkout, update, `go generate`, commit, pins commit, push, create PR |
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
