package utils

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func GetDirNameFromUrl(url string) string {
	name := nameFromUrl(url)
	lastSegment := name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		lastSegment = name[idx+1:]
	}
	bytes := sha256.Sum256([]byte(url))
	suffix := fmt.Sprintf("%x", bytes)[:8]
	return fmt.Sprintf("%s%s", lastSegment, suffix)
}

func nameFromUrl(url string) string {
	urlWithoutGitSuffix := strings.TrimSuffix(url, ".git")

	parts := strings.SplitN(urlWithoutGitSuffix, "/", 4)
	if len(parts) >= 4 {
		return parts[2] + "/" + parts[3]
	}
	if idx := strings.Index(urlWithoutGitSuffix, ":"); idx >= 0 {
		return urlWithoutGitSuffix[idx+1:]
	}
	return urlWithoutGitSuffix
}
