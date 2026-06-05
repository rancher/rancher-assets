# Release Team Guide

Quick reference for Release Team to manage rancher-assets releases.

## TL;DR

- **Auto prereleases** happen on every merge to main (for changed chart majors)
- **Manual releases** give you full control via GitHub Actions UI
- **Versions tracked** on orphan `versions` branch (not in main)
- **Git tags** trigger builds automatically

## Setup Requirements

### Required Secrets

The workflows require these GitHub secrets to be configured:

**`WORKFLOW_PAT`** (Required for automated releases)
- Personal Access Token - **MUST be a Classic token** (fine-grained tokens may not trigger workflows reliably)
- Scope: **`repo`** (Full control of private repositories)
- Used ONLY to push tags that trigger build workflows
- Without this, tags are created but builds won't trigger
- Fallback: Uses `GITHUB_TOKEN` (creates tags but doesn't trigger builds)
- Note: `GITHUB_TOKEN` is used for everything else (versions branch, etc.)

**To create WORKFLOW_PAT:**
1. GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate new token (classic)
3. Select scope: `repo` ✅
4. Copy token
5. Add to Repository Settings → Secrets → New repository secret → Name: `WORKFLOW_PAT`

**Important:** Fine-grained PATs may not work for triggering workflows. Use classic PAT with `repo` scope.

**`RANCHER_REPO_TOKEN`** (Optional - for rancher/rancher PRs)
- PAT with access to create PRs in `rancher/rancher`
- Only needed if you want automated PRs on stable releases
- If not set, stable releases won't create rancher/rancher PRs

**Configure in:** Repository Settings → Secrets and variables → Actions → New repository secret

## Quick Actions

### View Current Versions

```bash
git checkout versions
cat versions.yaml
git checkout main
```

Or via GitHub: https://github.com/rancher/rancher-assets/blob/versions/versions.yaml

### Trigger Manual Release

**GitHub UI:** Actions → Manual Release → Run workflow

**Common Scenarios:**

#### Release All Chart Majors (e.g., BCI security fix)
```
commit_sha: [leave empty for HEAD]
chart_major: [leave empty for ALL]
bump_type: patch
release_type: prerelease  (or stable if validated)
```

#### Release v1 Only (Rancher 2.15 releases)
```
commit_sha: [leave empty for HEAD]
chart_major: v1
bump_type: minor
release_type: stable
```

#### Release from Specific Commit
```
commit_sha: abc123def456
chart_major: v0
bump_type: patch
release_type: stable
```

## Release Types

### Prerelease (default)
- **When**: For testing, validation, RC builds
- **Format**: `v1.2.3-rc.1`, `v1.2.3-rc.2`, etc.
- **Auto**: Happens on merge to main
- **Manual**: Select "prerelease" in workflow

### Stable
- **When**: Production-ready releases
- **Format**: `v1.2.3` (clean semver)
- **Auto**: Never (must be manual)
- **Manual**: Select "stable" in workflow
- **Effect**: Creates PR to rancher/rancher

## Bump Types

### Patch
- `v1.2.3` → `v1.2.4`
- For bug fixes, CVE fixes, minor updates
- Most common

### Minor
- `v1.2.3` → `v1.3.0`
- For Rancher minor releases
- Coordinated with Rancher release schedule
- Each Rancher minor gets corresponding chart major minor bump

## Workflow Outputs

After triggering a manual release:

1. **Summary shows plan**
   - Which versions will be created
   - Which commit they'll use

2. **Tags created**
   - Visible in repository tags
   - Example: `v0.1.1`, `v1.2.3-rc.1`

3. **Build triggered**
   - Automatic via build-release.yml
   - Images pushed to Docker Hub
   - GitHub Release created

4. **PR created** (stable only)
   - Automatic PR to rancher/rancher
   - Updates `build.yaml` with new version

5. **Versions branch updated**
   - `versions.yaml` reflects new versions
   - Audit trail of releases

## Version Queries

### Latest stable for v1
```bash
git checkout versions
yq '.v1.stable.tag' versions.yaml
```

### Latest prerelease for v0
```bash
git checkout versions
yq '.v0.prerelease.tag' versions.yaml
```

### When was v1 last released?
```bash
git checkout versions
yq '.v1.stable.updated-at' versions.yaml
```

### All releases for v1
```bash
git tag -l "v1.*" --sort=-version:refname
```

## Decision Matrix

| Situation | chart_major | bump_type | release_type |
|-----------|-------------|-----------|--------------|
| Rancher 2.15 releases | `v1` | `minor` | `stable` |
| CVE fix affects all | `` (empty) | `patch` | `prerelease` → test → `stable` |
| Bug fix in v0 only | `v0` | `patch` | `stable` |
| Validate next minor | `v1` | `minor` | `prerelease` |
| Emergency patch from specific commit | `v0` | `patch` | `stable` + `commit_sha` |

## Safety Features

### Defaults
- `release_type` defaults to `prerelease` (safer)
- `commit_sha` defaults to HEAD (latest on main)
- `chart_major` empty = ALL (batch releases)

### Validation
- Workflow validates commit exists
- Shows plan before creating tags
- Creates tags atomically
- Updates versions branch after success

### Audit Trail
- Git tags are immutable
- Versions branch history shows all releases
- GitHub releases include full metadata
- PR to rancher/rancher creates review point

## Common Workflows

### Normal Rancher Release Cycle

```
1. Development work merges to main
   → Auto prereleases (v1.3.0-rc.1, v1.3.0-rc.2, ...)

2. Rancher 2.15 release approaching
   → Manual workflow:
     chart_major: v1
     bump_type: minor
     release_type: prerelease
   → Creates v1.3.0-rc.1 (if not already)

3. Validation in staging
   → Test v1.3.0-rc.1

4. Release Team approves
   → Manual workflow:
     chart_major: v1
     bump_type: minor
     release_type: stable
   → Creates v1.3.0
   → PR to rancher/rancher
   → Rancher 2.15 uses v1.3.0
```

### Emergency Security Fix

```
1. CVE discovered in BCI base image

2. PR created to bump BCI version
   → Affects all Dockerfiles

3. PR merges to main
   → Auto prereleases:
     v0.1.2-rc.1
     v1.2.4-rc.1

4. Quick validation of RC images

5. Release Team promotes to stable
   → Manual workflow:
     chart_major: [empty] ← ALL
     bump_type: patch
     release_type: stable
     commit_sha: [the merge commit]
   → Creates:
     v0.1.2
     v1.2.4
   → PRs to rancher/rancher for both branches
```

### Single Chart Major Patch

```
1. Bug found specific to v0 (Rancher 2.14)

2. Fix merged to main
   → Auto prerelease: v0.1.3-rc.1

3. Validation confirms fix

4. Release Team releases v0 only
   → Manual workflow:
     chart_major: v0
     bump_type: patch
     release_type: stable
   → Creates v0.1.3
   → PR to rancher/rancher (release/v2.14 branch)
```

## Troubleshooting

### "No changes detected" in auto-prerelease
- Check if dockerfiles/config/lock actually changed
- Auto-prerelease only triggers on specific paths
- Use manual workflow if needed

### "Commit not found"
- Verify commit SHA is correct
- Make sure commit exists in this repository
- Check that commit is on main branch

### "Tag already exists"
- Each version can only be tagged once
- Check existing tags: `git tag -l "v1.*"`
- Choose next version number

### Build fails
- Check build-release.yml workflow logs
- Verify Dockerfile.vN exists and is valid
- Check lock.yaml has required upstream refs

### PR to rancher/rancher not created
- Only happens for stable releases
- Check RANCHER_REPO_TOKEN secret is configured
- Verify rancher-branch in config.yaml is correct

## Monitoring

### Watch for Auto Prereleases
- **Trigger**: Merges to main
- **Where**: Actions → Auto Pre-release
- **What**: Shows which majors changed, versions created

### Check Build Status
- **Trigger**: Tag creation
- **Where**: Actions → Build and Release
- **What**: Build progress, image push status

### Verify Versions Branch
- **Where**: https://github.com/rancher/rancher-assets/blob/versions/versions.yaml
- **What**: Current stable and prerelease versions

### GitHub Releases
- **Where**: Releases tab
- **What**: Published versions with metadata

## Contact

For questions or issues:
- Open issue in rancher/rancher-assets
- Tag @rancher/release-team
