package imagelist

import (
	"fmt"
	"slices"
	"strings"
)

// validateImage checks if an image reference is valid
// Returns an error describing the issue if invalid, nil if valid
func validateImage(image string) error {
	if image == "" {
		return fmt.Errorf("empty image reference")
	}

	// Check for placeholder patterns commonly used in chart values
	placeholderPatterns := []struct {
		pattern string
		desc    string
	}{
		{"$<", "placeholder syntax ($<...>)"},
		{"<IMAGE>", "IMAGE placeholder"},
		{"<image>", "image placeholder"},
		{"${IMAGE}", "IMAGE variable placeholder"},
		{"${image}", "image variable placeholder"},
		{"REPLACE_ME", "REPLACE_ME placeholder"},
		{"TODO", "TODO placeholder"},
		{"FIXME", "FIXME placeholder"},
		{"YOUR_", "YOUR_* placeholder"},
		{"<INSERT", "INSERT placeholder"},
		{"CHANGEME", "CHANGEME placeholder"},
	}

	for _, p := range placeholderPatterns {
		if strings.Contains(image, p.pattern) {
			return fmt.Errorf("contains %s", p.desc)
		}
	}

	// Check for unclosed template syntax (these should be caught earlier but double-check)
	if strings.Contains(image, "{{") || strings.Contains(image, "}}") {
		return fmt.Errorf("contains template syntax")
	}

	// Check for obvious invalid characters in image names
	// Valid characters: alphanumeric, ., _, -, /, :, @
	invalidChars := []string{"$", "<", ">", "{", "}", " ", "`", "'", "\"", "\\", "|", "&", ";"}
	for _, char := range invalidChars {
		if strings.Contains(image, char) {
			return fmt.Errorf("contains invalid character '%s'", char)
		}
	}

	// Basic structure validation: should have at least a name
	// Valid formats:
	//   - image:tag
	//   - registry/image:tag
	//   - registry/namespace/image:tag
	//   - image@digest
	//   - registry/image@digest
	parts := strings.Split(image, "/")
	if len(parts) == 0 {
		return fmt.Errorf("malformed image reference")
	}

	// The last part should contain the image name and optionally :tag or @digest
	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return fmt.Errorf("empty image name")
	}

	// Check if it has a tag or digest
	hasTag := strings.Contains(lastPart, ":")
	hasDigest := strings.Contains(lastPart, "@")

	// Warn if it has neither (though this might be valid in some contexts, it's suspicious)
	if !hasTag && !hasDigest {
		// Allow this for now, but could be flagged as a warning
		// return fmt.Errorf("missing tag or digest")
	}

	return nil
}

// addInvalidImage tracks an invalid image with its context
func addInvalidImage(invalidSet InvalidImageSet, image, reason, source, catalogRef string) {
	key := image + "|" + reason // Unique key per image+reason combination

	if entry, exists := invalidSet[key]; exists {
		// Add source to existing entry if not already present
		entry.Sources = appendUnique(entry.Sources, source)
	} else {
		// Create new entry
		invalidSet[key] = &InvalidImageEntry{
			Image:      image,
			Reason:     reason,
			Sources:    []string{source},
			CatalogRef: catalogRef,
		}
	}
}

// appendUnique adds a string to a slice if not already present
func appendUnique(slice []string, s string) []string {
	if slices.Contains(slice, s) {
		return slice
	}
	return append(slice, s)
}
