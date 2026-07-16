# Image List Generation TODOs

This document tracks known issues and areas for improvement in the image list generation code.

## Critical Issues

### 1. Tag-Only Image References (CRITICAL BUG)
**Status**: BROKEN - produces invalid output
**Location**: `scanner.go:224-243` (formatImageReference), `validation.go:60-83` (validateImage)

**Examples from real output**:
- `:v3.13.0` (rancher-charts)
- `:latest`, `:2.2.16`, `:v1.0.0` (rancher-partner-charts)

The scanner currently allows image references that consist of only a tag (e.g., `:v3.13.0`) without a repository/image name. This happens when:
- helmtools extracts an Image with empty Repository field but valid Tag
- The formatImageReference function concatenates registry/repository/tag, producing just `:tag` when repository is empty
- The validation.go validateImage function doesn't catch this case

**Impact**:
- Generated image lists contain invalid references that cannot be pulled
- Downstream tools (docker pull, mirroring scripts) will fail on these entries
- Airgap scenarios will be missing critical images

**TODO**:
- [ ] Fix validation to reject tag-only references (e.g., must not start with `:`)
- [ ] Add validation that image has at least a repository name before tag/digest
- [ ] Track tag-only references in invalid-images.txt report
- [ ] Investigate source charts to understand why Repository is empty
- [ ] Consider if we should error/warn during scanning vs. just filtering

### 2. Charts with Missing or Invalid Tag Values
**Status**: Detected but not comprehensive
**Location**: `validation.go:76-80`

Currently, images without tags or digests are allowed through validation. This can lead to:
- Images with implicit `:latest` tags being included
- Incomplete image references that may fail during mirroring

**TODO**:
- [ ] Add warning when images lack explicit tags/digests
- [ ] Track these cases in a separate report file
- [ ] Consider making explicit tags mandatory for production image lists

### 2. Template Variable Handling
**Status**: Partial - templates are skipped
**Location**: `scanner.go:277-280`

Images containing template variables (e.g., `{{ .Values.image }}`) are silently skipped. This means:
- We can't detect what the actual runtime image will be
- No tracking of which charts use templated images
- Potential missing images in mirror lists

**TODO**:
- [ ] Create a report of charts using templated images
- [ ] Document which charts require runtime value substitution
- [ ] Consider parsing common template patterns to extract default values

### 3. Placeholder Image Detection
**Status**: Basic validation only
**Location**: `validation.go:16-38`

Current placeholder detection catches common patterns but may miss:
- Custom placeholder conventions
- Commented-out image references
- Environment-specific placeholders

**TODO**:
- [ ] Expand placeholder pattern list based on real chart analysis
- [ ] Add heuristics for detecting "example" or "test" image references
- [ ] Categorize placeholders by severity (definitely invalid vs. suspicious)

## Scanning Coverage Issues

### 4. CRD Charts vs. Non-CRD Charts
**Status**: Unknown - needs investigation
**Location**: `scanner.go:194-219` (extractImagesFromChart)

Current implementation:
- Only scans `values.yaml` files
- May miss images defined in CRD specs, templates, or other locations
- No differentiation between chart types

**TODO**:
- [ ] Investigate if CRD charts store images differently than regular charts
- [ ] Check if images can be defined in:
  - Chart templates (e.g., `templates/deployment.yaml`)
  - CRD specifications
  - `Chart.yaml` annotations
  - Sub-chart values
- [ ] Add scanning for additional locations if needed

### 5. Sub-chart Image References
**Status**: Unknown - needs investigation
**Location**: `scanner.go:194-219`

**TODO**:
- [ ] Determine if sub-chart dependencies are scanned
- [ ] Verify that images from sub-charts are included in parent chart results
- [ ] Handle cases where sub-charts override parent values

### 6. Chart Version Selection
**Status**: Only scans latest version
**Location**: `scanner.go:138-142`

Currently only the first (latest) version of each chart is scanned. This means:
- Historical versions are ignored
- No detection of image changes across versions

**TODO**:
- [ ] Document why only latest version is scanned
- [ ] Consider if older versions need to be supported for rollbacks
- [ ] Add option to scan specific versions or version ranges

## Validation and Quality Issues

### 7. OS Detection Accuracy
**Status**: Basic heuristic
**Location**: `scanner.go:313-317`

Current OS detection only checks for "windows" or "nanoserver" in image names. This:
- May miss Windows images with different naming
- Can't detect multi-arch images
- No validation against actual image manifests

**TODO**:
- [ ] Improve OS detection heuristics
- [ ] Consider querying registry for actual image platform information
- [ ] Handle multi-arch images that support both Linux and Windows

### 8. Image Reference Normalization
**Status**: Minimal
**Location**: `scanner.go:283`

Current normalization only trims whitespace. Missing:
- Registry hostname normalization (docker.io vs registry-1.docker.io)
- Default tag handling (no tag vs :latest)
- Digest format validation

**TODO**:
- [ ] Implement comprehensive image reference normalization
- [ ] Standardize registry hostnames to canonical forms
- [ ] Detect and flag images using :latest tag

### 9. Invalid Character Detection
**Status**: Basic
**Location**: `validation.go:47-52`

Current validation blocks obvious invalid characters but may miss:
- Unicode characters in edge cases
- Invalid registry hostname formats
- Port number validation

**TODO**:
- [ ] Use proper OCI image reference parsing library
- [ ] Validate registry hostname format
- [ ] Ensure compliance with OCI distribution spec

## Feature Gaps

### 10. Image Deduplication Across Catalogs
**Status**: Not implemented
**Location**: `scanner.go:41-89`

Each catalog is processed independently. This means:
- No detection of duplicate images across catalogs
- No way to generate a combined image list
- Inefficient for mirroring scenarios

**TODO**:
- [ ] Add option to generate deduplicated combined image list
- [ ] Report which images appear in multiple catalogs
- [ ] Optimize for airgap scenarios where all images are needed

### 11. Old Chart Filtering
**Status**: Placeholder exists but not implemented
**Location**: `scanner.go:35-37` (skipOldCharts)

The `skipOldCharts` function currently returns false for all charts. This was intended to:
- Filter out EOL chart versions
- Reduce image list size for old/unsupported releases
- Focus on actively maintained charts

**TODO**:
- [ ] Define criteria for "old" charts (e.g., older than X releases)
- [ ] Implement skipOldCharts logic based on version semantics
- [ ] Make this configurable per catalog
- [ ] Document which charts are being skipped and why

### 12. Image Source Tracking
**Status**: Implemented but not exported
**Location**: `types.go:4-8`

Source tracking exists in the ImageReference struct but is not included in the main output files. This means:
- No way to trace which chart(s) use each image
- Difficult to debug missing or unexpected images
- Lost context for image provenance

**TODO**:
- [ ] Consider adding an optional detailed report with source information
- [ ] Add command-line flag to enable verbose output
- [ ] Include source tracking in invalid image reports for debugging

## Testing and Validation

### 13. Missing Test Coverage
**Status**: No tests exist
**Location**: Entire package

**TODO**:
- [ ] Add unit tests for validation functions
- [ ] Add integration tests with sample chart catalogs
- [ ] Test edge cases (empty charts, malformed YAML, etc.)
- [ ] Verify output format stability

### 14. Output Validation
**Status**: No validation
**Location**: `scanner.go:357-414` (WriteImageLists)

Generated image lists are not validated after creation. Should verify:
- All images are syntactically valid
- No duplicate entries within a file
- Files are sorted consistently
- Proper line endings and encoding

**TODO**:
- [ ] Add post-generation validation
- [ ] Verify image list can be consumed by downstream tools
- [ ] Add checksums or signatures for output files

## Performance and Scalability

### 15. Catalog Scanning Performance
**Status**: Sequential processing
**Location**: `scanner.go:59-86`

Catalogs are scanned sequentially. For large catalogs this can be slow.

**TODO**:
- [ ] Profile scanning performance on large catalogs
- [ ] Consider parallel chart processing within a catalog
- [ ] Add progress reporting for long-running scans
- [ ] Cache parsed index.yaml to speed up repeated scans

### 16. Memory Usage
**Status**: All images held in memory
**Location**: `scanner.go:70-85`

All image references are accumulated in memory before writing output.

**TODO**:
- [ ] Profile memory usage on large catalogs
- [ ] Consider streaming output for very large image sets
- [ ] Monitor memory usage during CI/CD runs

## Documentation

### 17. Output Format Documentation
**Status**: Minimal
**Location**: Only in invalid-images.txt header

**TODO**:
- [ ] Document expected output file format
- [ ] Provide examples of each output file type
- [ ] Document file naming conventions
- [ ] Add README explaining how to use generated lists

### 18. Catalog Structure Assumptions
**Status**: Implicit in code
**Location**: `scanner.go:91-118` (findCatalogDir)

Code makes assumptions about catalog directory structure (hash suffixes, index.yaml location).

**TODO**:
- [ ] Document expected catalog directory structure
- [ ] Add validation that catalogs conform to expected structure
- [ ] Provide better error messages when structure doesn't match

---

## Priority Legend
- **Critical**: Blocks basic functionality or produces incorrect results
- **High**: Significantly impacts usability or reliability
- **Medium**: Improves functionality but not critical
- **Low**: Nice to have improvements

Current priorities:
1. Critical: Items 1, 4
2. High: Items 2, 3, 11
3. Medium: Items 7, 8, 10, 12
4. Low: Items 5, 6, 9, 13-18
