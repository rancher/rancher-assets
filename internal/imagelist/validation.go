package imagelist

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ValidationWarningError represents a validation issue that doesn't prevent image usage
// but should be flagged to the user
type ValidationWarningError struct {
	message string
}

func (w ValidationWarningError) Error() string {
	return w.message
}

// IsWarning checks if an error is a validation warning rather than a hard error
func IsWarning(err error) bool {
	var warning ValidationWarningError
	return errors.As(err, &warning)
}

// checkForPlaceholders checks if an image reference contains placeholder patterns
// Returns an error if a placeholder is found, nil otherwise
func checkForPlaceholders(image string) error {
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
	return nil
}

// checkForInvalidCharacters checks if an image reference contains invalid characters
// Returns an error if invalid characters are found, nil otherwise
func checkForInvalidCharacters(image string) error {
	// Check for unclosed template syntax
	if strings.Contains(image, "{{") || strings.Contains(image, "}}") {
		return errors.New("contains template syntax")
	}

	// Check for obvious invalid characters in image names
	// Valid characters: alphanumeric, ., _, -, /, :, @
	invalidChars := []string{"$", "<", ">", "{", "}", " ", "`", "'", "\"", "\\", "|", "&", ";"}
	for _, char := range invalidChars {
		if strings.Contains(image, char) {
			return fmt.Errorf("contains invalid character '%s'", char)
		}
	}
	return nil
}

// validateImage checks if an image reference is valid
// Returns an error describing the issue if invalid, nil if valid
func validateImage(image string) error {
	if image == "" {
		return errors.New("empty image reference")
	}

	// Check for tag-only references (e.g., ":v1.0.0" without repository)
	if strings.HasPrefix(image, ":") {
		return errors.New("tag-only reference (missing repository/image name)")
	}

	// Check for digest-only references (e.g., "@sha256:..." without repository)
	if strings.HasPrefix(image, "@") {
		return errors.New("digest-only reference (missing repository/image name)")
	}

	// Check for placeholder patterns commonly used in chart values
	if err := checkForPlaceholders(image); err != nil {
		return err
	}

	// Check for invalid characters
	if err := checkForInvalidCharacters(image); err != nil {
		return err
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
		return errors.New("malformed image reference")
	}

	// The last part should contain the image name and optionally :tag or @digest
	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return errors.New("empty image name")
	}

	// Ensure there's an image name before the tag or digest separator
	// Split on both : and @ to get the base image name
	imageName := lastPart
	if idx := strings.IndexAny(imageName, ":@"); idx != -1 {
		imageName = imageName[:idx]
	}
	if imageName == "" {
		return errors.New("missing image name before tag/digest")
	}

	// Check if it has a tag or digest
	hasTag := strings.Contains(lastPart, ":")
	hasDigest := strings.Contains(lastPart, "@")

	// Warn if it has neither (technically valid but will default to :latest)
	if !hasTag && !hasDigest {
		return ValidationWarningError{message: "missing tag or digest (will default to :latest)"}
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
