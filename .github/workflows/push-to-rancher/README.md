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
1. Clone rancher/rancher. If the bot already has a PR open for the branch, its branch is checked out and the base branch is merged into it ([Updating a PR that is still open](#updating-a-pr-that-is-still-open)); otherwise a new branch is created
2. Update `defaultAssetsImage` in `build.yaml`
3. Run `go generate ./pkg/...` to update generated files
4. Commit changes
5. For a dev image of a branch on the staging catalog, update the chart pins and commit them as a second commit (see [Chart pins](#chart-pins))
6. Push the branch, and create the PR, or update the title and the body of the open one

## Updating a PR that is still open

A new image often arrives while the PR of the previous one is not merged yet (new charts were staged in between). Instead of opening another PR next to it, the bot updates the open one.

- **Which PR.** `find_open_bot_pr` (in `common.sh`) takes the newest open PR of the base branch whose head branch starts with `bot/rancher-assets-` and, when `APP_USER` is set, whose author is that app. Older bot PRs that are still open are left alone.
- **How.** The branch of that PR is fetched and checked out, and the base branch is **merged into it** (never rebased: there is no force push). Then the image, `go generate`, and the pins are done as for a new PR, as new commits on top, and pushed as a fast-forward. The title (it has the tag in it) and the body of the PR are updated with `gh pr edit`.
- **Conflicts are not handled one by one.** The lines that conflict with the base are the ones the bot sets (the image and the pins), so the merge uses `-X theirs`: the base wins, and the commits that follow set those lines again. Only adding the new commits would not remove a conflict: a line that both sides changed stays in conflict, because the common ancestor is the old one.
- **Shallow clones.** The clone of rancher/rancher in Actions has one commit of the base branch. If the base moved after the PR was opened, the commit where the branch started is not in it and git refuses to merge ("refusing to merge unrelated histories"). So, in a shallow clone, the base and the branch are fetched with `--shallow-since` a day before the PR was created (`find_open_bot_pr` prints the creation time as its third field), which brings a few commits and no noticeable size. A complete clone is not touched: asking it for a shallow fetch would make it shallow.
- **If it cannot.** If the branch cannot be fetched, or the merge fails anyway, the PR is left as it is, a warning goes to the summary, and a new PR is opened, as it was before this existed. The same happens if `gh` cannot list the PRs.
- **The body** lists what the whole branch moved against the base branch, not only what the last run moved.
- **The same tag again** changes nothing: the image is already up to date, nothing is pushed and the PR is not edited.

What it costs: every update is a push to the branch, so the checks of rancher/rancher run again, and a review of the PR may have to be repeated if the repository dismisses reviews on new commits. The PR also grows by the commits of each update, which a squash merge hides.

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

**Tests.** `tests/update-pins_test.sh` runs the script against fixtures, and `tests/find_open_bot_pr_test.sh` tests the search of the open PR with a stub of `gh`; both need no network and no clone, and `scripts/test` runs them in CI. The update of an open PR as a whole (the merge, the conflicts, the title and the body) was tried against a local stand-in remote and a stub of `gh`, with the real index, for a first run, a base branch that moved with conflicts, an update, and a repeated run.

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
