package proofstorage

import (
	"mime"
	"net/http"
	"path"
	"strings"
)

func extensionFromResponse(sourceURL, contentType string) string {
	if ext := extensionFromContentType(contentType); ext != "" {
		return ext
	}
	if ext := path.Ext(sourceURL); ext != "" {
		switch strings.ToLower(ext) {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif":
			return ext
		}
	}
	return ".png"
}

func extensionFromContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch mediaType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		if strings.HasPrefix(mediaType, "image/") {
			if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
				return exts[0]
			}
		}
		return ""
	}
}

func isSuccessStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
