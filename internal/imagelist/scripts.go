package imagelist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Script templates for saving/loading/mirroring images

const bashSaveScript = `#!/bin/bash
set -e

usage() {
    echo "Usage: $0 --image-list <file>"
    exit 1
}

IMAGE_LIST=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --image-list)
            IMAGE_LIST="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

if [ -z "$IMAGE_LIST" ]; then
    usage
fi

if [ ! -f "$IMAGE_LIST" ]; then
    echo "Error: Image list file not found: $IMAGE_LIST"
    exit 1
fi

OUTPUT_TAR="rancher-charts-images.tar.gz"

echo "Pulling images from $IMAGE_LIST..."
while IFS= read -r image; do
    echo "  Pulling $image..."
    docker pull "$image"
done < "$IMAGE_LIST"

echo "Saving images to $OUTPUT_TAR..."
docker save $(cat "$IMAGE_LIST") | gzip > "$OUTPUT_TAR"

echo "Done! Images saved to $OUTPUT_TAR"
echo "Size: $(du -h $OUTPUT_TAR | cut -f1)"
`

const bashLoadScript = `#!/bin/bash
set -e

usage() {
    echo "Usage: $0 --image-list <file>"
    exit 1
}

IMAGE_LIST=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --image-list)
            IMAGE_LIST="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

if [ -z "$IMAGE_LIST" ]; then
    usage
fi

TAR_FILE="rancher-charts-images.tar.gz"

if [ ! -f "$TAR_FILE" ]; then
    echo "Error: Image archive not found: $TAR_FILE"
    exit 1
fi

echo "Loading images from $TAR_FILE..."
docker load -i "$TAR_FILE"

echo "Done! Images loaded from $TAR_FILE"
`

const bashMirrorScript = `#!/bin/bash
set -e

usage() {
    echo "Usage: $0 --image-list <file> --registry <registry>"
    exit 1
}

IMAGE_LIST=""
REGISTRY=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --image-list)
            IMAGE_LIST="$2"
            shift 2
            ;;
        --registry)
            REGISTRY="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

if [ -z "$IMAGE_LIST" ] || [ -z "$REGISTRY" ]; then
    usage
fi

if [ ! -f "$IMAGE_LIST" ]; then
    echo "Error: Image list file not found: $IMAGE_LIST"
    exit 1
fi

echo "Mirroring images to $REGISTRY..."
while IFS= read -r image; do
    # Strip any existing registry prefix
    image_without_registry=$(echo "$image" | sed 's|^[^/]*/||')
    target_image="$REGISTRY/$image_without_registry"

    echo "  $image -> $target_image"
    docker pull "$image"
    docker tag "$image" "$target_image"
    docker push "$target_image"
done < "$IMAGE_LIST"

echo "Done! Images mirrored to $REGISTRY"
`

const powershellSaveScript = `param(
    [Parameter(Mandatory=$true)]
    [string]$ImageList
)

if (-not (Test-Path $ImageList)) {
    Write-Error "Image list file not found: $ImageList"
    exit 1
}

$outputTar = "rancher-charts-windows-images.tar.gz"

Write-Host "Pulling images from $ImageList..."
Get-Content $ImageList | ForEach-Object {
    $image = $_.Trim()
    if ($image) {
        Write-Host "  Pulling $image..."
        docker pull $image
    }
}

Write-Host "Saving images to $outputTar..."
$images = Get-Content $ImageList | Where-Object { $_ -ne "" }
docker save $images | gzip > $outputTar

Write-Host "Done! Images saved to $outputTar"
$size = (Get-Item $outputTar).Length / 1GB
Write-Host "Size: $([math]::Round($size, 2)) GB"
`

const powershellLoadScript = `param(
    [Parameter(Mandatory=$true)]
    [string]$ImageList
)

$tarFile = "rancher-charts-windows-images.tar.gz"

if (-not (Test-Path $tarFile)) {
    Write-Error "Image archive not found: $tarFile"
    exit 1
}

Write-Host "Loading images from $tarFile..."
docker load -i $tarFile

Write-Host "Done! Images loaded from $tarFile"
`

const powershellMirrorScript = `param(
    [Parameter(Mandatory=$true)]
    [string]$ImageList,

    [Parameter(Mandatory=$true)]
    [string]$Registry
)

if (-not (Test-Path $ImageList)) {
    Write-Error "Image list file not found: $ImageList"
    exit 1
}

Write-Host "Mirroring images to $Registry..."
Get-Content $ImageList | ForEach-Object {
    $image = $_.Trim()
    if ($image) {
        # Strip any existing registry prefix
        $imageWithoutRegistry = $image -replace '^[^/]*/', ''
        $targetImage = "$Registry/$imageWithoutRegistry"

        Write-Host "  $image -> $targetImage"
        docker pull $image
        docker tag $image $targetImage
        docker push $targetImage
    }
}

Write-Host "Done! Images mirrored to $Registry"
`

// WriteImageLists generates all output files for a set of image references
func WriteImageLists(refs []ImageReference, config ExportConfig) error {
	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return err
	}

	// Separate Linux and Windows images
	var linuxImages, windowsImages []ImageReference
	for _, ref := range refs {
		if ref.OS == Windows {
			windowsImages = append(windowsImages, ref)
		} else {
			linuxImages = append(linuxImages, ref)
		}
	}

	// Generate Linux files
	if err := writeImageFiles("rancher-charts", linuxImages, config.OutputDir); err != nil {
		return err
	}

	// Generate Windows files
	if err := writeImageFiles("rancher-charts-windows", windowsImages, config.OutputDir); err != nil {
		return err
	}

	// Generate image origins file (combined)
	if err := writeImageOrigins(refs, config.OutputDir); err != nil {
		return err
	}

	fmt.Printf("\nGenerated image lists in %s/:\n", config.OutputDir)
	fmt.Printf("  - rancher-charts-images.txt (%d images)\n", len(linuxImages))
	fmt.Printf("  - rancher-charts-windows-images.txt (%d images)\n", len(windowsImages))
	fmt.Printf("  - rancher-charts-image-origins.txt\n")
	fmt.Printf("  - Helper scripts (save/load/mirror)\n")

	return nil
}

// writeImageFiles writes image list and scripts for a specific OS
func writeImageFiles(prefix string, images []ImageReference, outputDir string) error {
	// Image list (simple list of images)
	imageListPath := filepath.Join(outputDir, prefix+"-images.txt")
	var imageList []string
	for _, img := range images {
		imageList = append(imageList, img.Image)
	}
	if err := writeLines(imageListPath, imageList); err != nil {
		return err
	}

	// Image sources (image + sources)
	sourcesPath := filepath.Join(outputDir, prefix+"-images-sources.txt")
	var sources []string
	for _, img := range images {
		sources = append(sources, fmt.Sprintf("%s %s", img.Image, strings.Join(img.Sources, " ")))
	}
	if err := writeLines(sourcesPath, sources); err != nil {
		return err
	}

	// Determine script type
	isWindows := strings.Contains(prefix, "windows")

	// Save script
	saveScriptPath := filepath.Join(outputDir, prefix+"-save-images."+scriptExt(isWindows))
	saveScript := bashSaveScript
	if isWindows {
		saveScript = powershellSaveScript
	}
	if err := os.WriteFile(saveScriptPath, []byte(saveScript), 0755); err != nil {
		return err
	}

	// Load script
	loadScriptPath := filepath.Join(outputDir, prefix+"-load-images."+scriptExt(isWindows))
	loadScript := bashLoadScript
	if isWindows {
		loadScript = powershellLoadScript
	}
	if err := os.WriteFile(loadScriptPath, []byte(loadScript), 0755); err != nil {
		return err
	}

	// Mirror script
	mirrorScriptPath := filepath.Join(outputDir, prefix+"-mirror-to-rancher-org."+scriptExt(isWindows))
	mirrorScript := bashMirrorScript
	if isWindows {
		mirrorScript = powershellMirrorScript
	}
	if err := os.WriteFile(mirrorScriptPath, []byte(mirrorScript), 0755); err != nil {
		return err
	}

	return nil
}

// writeImageOrigins writes a combined image origins file
func writeImageOrigins(refs []ImageReference, outputDir string) error {
	path := filepath.Join(outputDir, "rancher-charts-image-origins.txt")

	var lines []string
	for _, ref := range refs {
		for _, source := range ref.Sources {
			lines = append(lines, fmt.Sprintf("%s %s", ref.Image, source))
		}
	}

	return writeLines(path, lines)
}

// writeLines writes a slice of strings to a file, one per line
func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n" // Ensure trailing newline
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// scriptExt returns the script file extension
func scriptExt(isWindows bool) string {
	if isWindows {
		return "ps1"
	}
	return "sh"
}
