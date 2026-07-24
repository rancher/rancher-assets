# Release Team Guide

Quick reference for Release Team to manage rancher-assets releases.

## TL;DR

- **Auto prereleases** happen on every merge to main (for changed Rancher minors, with -dev suffix)
- **Manual releases** give you full control via GitHub Actions UI
- **Versions tracked** via Git tags with CalVer timestamps
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
# Latest stable for Rancher 2.14
git tag -l "v2.14-*" --sort=-version:refname | grep -v -- '-dev$' | head -1

# Latest stable for Rancher 2.15
git tag -l "v2.15-*" --sort=-version:refname | grep -v -- '-dev$' | head -1

# All versions for Rancher 2.15
git tag -l "v2.15-*" --sort=-version:refname
```

### Trigger Manual Release

**GitHub UI:** Actions → Manual Release → Run workflow

**Common Scenarios:**

#### Release All Rancher Minors (e.g., BCI security fix)
```
commit_sha: [leave empty for HEAD]
rancher_minor: [leave empty for ALL]
release_type: prerelease  (or stable if validated)
```

#### Release Rancher 2.15 Only
```
commit_sha: [leave empty for HEAD]
rancher_minor: 2.15
release_type: stable
```

#### Release from Specific Commit
```
commit_sha: abc123def456
rancher_minor: 2.14
release_type: stable
```

## Release Types

### Prerelease (default)
- **When**: For testing, validation, RC builds
- **Format**: `v2.15-20260716T1430Z-dev` (with -dev suffix)
- **Auto**: Happens on merge to main
- **Manual**: Select "prerelease" in workflow

### Stable
- **When**: Production-ready releases
- **Format**: `v2.15-20260805T1600Z` (no -dev suffix)
- **Auto**: Never (must be manual)
- **Manual**: Select "stable" in workflow
- **Effect**: Creates PR to rancher/rancher

## Version Format

**CalVer with ISO 8601 Timestamp:**

`v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`

- **RANCHER_MINOR**: Rancher minor version (e.g., 2.14, 2.15)
- **YYYYMMDD**: Date in UTC
- **HHMM**: Time in UTC (24-hour)
- **Z**: Zulu time (UTC timezone indicator)
- **-dev**: Suffix for pre-releases

**Examples:**
- `v2.14-20260716T1430Z` - Stable release on July 16, 2026 at 14:30 UTC
- `v2.15-20260805T1600Z-dev` - Pre-release on August 5, 2026 at 16:00 UTC

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

## Version Queries

### Latest stable for Rancher 2.15
```bash
git tag -l "v2.15-*" --sort=-version:refname | grep -v -- '-dev$' | head -1
```

### Latest prerelease for Rancher 2.14
```bash
git tag -l "v2.14-*" --sort=-version:refname | grep -- '-dev$' | head -1
```

### When was a version released?
```bash
git log -1 --format='%ai' v2.15-20260805T1600Z
```

### All releases for Rancher 2.15
```bash
git tag -l "v2.15-*" --sort=-version:refname
```

## Decision Matrix

| Situation | rancher_minor | release_type |
|-----------|---------------|--------------|
| Rancher 2.15 GA | `2.15` | `stable` |
| CVE fix affects all | `` (empty) | `prerelease` → test → `stable` |
| Bug fix in 2.14 only | `2.14` | `stable` |
| Validate upcoming release | `2.15` | `prerelease` |
| Emergency patch from specific commit | `2.14` | `stable` + `commit_sha` |

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
   → Auto prereleases (v2.15-20260716T1430Z-dev, v2.15-20260717T0930Z-dev, ...)

2. Rancher 2.15 release approaching
   → Manual workflow:
     rancher_minor: 2.15
     release_type: prerelease
   → Creates v2.15-20260801T1045Z-dev

3. Validation in staging
   → Test v2.15-20260801T1045Z-dev

4. Release Team approves
   → Manual workflow:
     rancher_minor: 2.15
     release_type: stable
   → Creates v2.15-20260805T1600Z
   → PR to rancher/rancher
   → Rancher 2.15 uses v2.15-20260805T1600Z
```

### Emergency Security Fix

```
1. CVE discovered in BCI base image

2. PR created to bump BCI version
   → Affects all Dockerfiles

3. PR merges to main at 14:30 UTC
   → Auto prereleases:
     v2.14-20260716T1430Z-dev
     v2.15-20260716T1430Z-dev

4. Quick validation of RC images

5. Release Team promotes to stable at 16:00 UTC
   → Manual workflow:
     rancher_minor: [empty] ← ALL
     release_type: stable
     commit_sha: [the merge commit]
   → Creates:
     v2.14-20260716T1600Z
     v2.15-20260716T1600Z
   → PRs to rancher/rancher for both branches
```

### Single Rancher Minor Patch

```
1. Bug found specific to Rancher 2.14

2. Fix merged to main
   → Auto prerelease: v2.14-20260722T0915Z-dev

3. Validation confirms fix

4. Release Team releases Rancher 2.14 only
   → Manual workflow:
     rancher_minor: 2.14
     release_type: stable
   → Creates v2.14-20260722T1100Z
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

### Verify Release Tags
- **Where**: Repository Tags tab, or `git tag -l`
- **What**: All released versions (stable and prerelease)

### GitHub Releases
- **Where**: Releases tab
- **What**: Published versions with metadata

## Contact

For questions or issues:
- Open issue in rancher/rancher-assets
- Tag @rancher/release-team
