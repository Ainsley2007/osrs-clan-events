package proofstorage

import "testing"

func TestObjectKey(t *testing.T) {
	if got := ObjectKey(4821, ".png"); got != "4821.png" {
		t.Fatalf("ObjectKey() = %q, want 4821.png", got)
	}
}

func TestPublicURL(t *testing.T) {
	tests := []struct {
		base string
		key  string
		want string
	}{
		{"https://pub.example.r2.dev", "4821.png", "https://pub.example.r2.dev/4821.png"},
		{"https://pub.example.r2.dev/", "4821.png", "https://pub.example.r2.dev/4821.png"},
	}
	for _, tc := range tests {
		if got := PublicURL(tc.base, tc.key); got != tc.want {
			t.Fatalf("PublicURL(%q, %q) = %q, want %q", tc.base, tc.key, got, tc.want)
		}
	}
}

func TestExtensionFromContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"image/png", ".png"},
		{"image/jpeg; charset=binary", ".jpg"},
		{"image/webp", ".webp"},
		{"text/plain", ""},
	}
	for _, tc := range tests {
		if got := extensionFromContentType(tc.contentType); got != tc.want {
			t.Fatalf("extensionFromContentType(%q) = %q, want %q", tc.contentType, got, tc.want)
		}
	}
}

func TestExtensionFromResponse(t *testing.T) {
	if got := extensionFromResponse("https://cdn.discordapp.com/attachments/1/2/proof.webp", ""); got != ".webp" {
		t.Fatalf("extensionFromResponse() = %q, want .webp", got)
	}
	if got := extensionFromResponse("https://cdn.discordapp.com/attachments/1/2/proof", "image/png"); got != ".png" {
		t.Fatalf("extensionFromResponse() with content type = %q, want .png", got)
	}
}
