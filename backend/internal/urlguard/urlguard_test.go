package urlguard

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		list *Allowlist
		url  string
		ok   bool
	}{
		{GoogleDrive, "https://www.googleapis.com/drive/v3/files/abc?alt=media&sz=s400", true},
		{GoogleDrive, "https://lh3.googleusercontent.com/abc=s400", true},
		{GoogleDrive, "https://WWW.GOOGLEAPIS.COM/drive/v3/files/abc", true},
		{GoogleDrive, "https://www.googleapis.com:443/drive/v3/files/abc", true},
		{OneDrive, "https://graph.microsoft.com/v1.0/shares/u!abc/driveItem", true},
		{OneDrive, "https://my.microsoftpersonalcontent.com/personal/abc/_layouts/15/download.aspx?UniqueId=x&tempauth=y", true},
		{OneDrive, "https://public.am.files.1drv.com/y4m/photo.jpg", true},
		{OneDrive, "https://contoso-my.sharepoint.com/_layouts/15/download.aspx?x=1", true},
		{OneDrive, "https://southcentralus1-mediap.svc.ms/transform/thumbnail?x=1", true},

		// Internal targets
		{GoogleDrive, "http://face-service:8081/face/session/x", false},
		{GoogleDrive, "https://169.254.169.254/latest/meta-data/", false},
		{GoogleDrive, "https://[::1]/", false},
		{OneDrive, "https://127.0.0.1/", false},
		// Look-alike hosts and tricks
		{GoogleDrive, "https://evil.example/?googleapis.com", false},
		{GoogleDrive, "https://www.googleapis.com.evil.example/", false},
		{GoogleDrive, "https://www.googleapis.com@evil.example/", false},
		{GoogleDrive, "https://user:pass@www.googleapis.com/", false},
		{GoogleDrive, "https://notgoogleusercontent.com/", false},
		{GoogleDrive, "https://googleapis.com/", false},
		{OneDrive, "https://evilsharepoint.com/", false},
		{OneDrive, "https://graph.microsoft.com.evil.example/", false},
		// Scheme and port
		{GoogleDrive, "http://www.googleapis.com/drive/v3/files/abc", false},
		{GoogleDrive, "https://www.googleapis.com:8443/", false},
		{GoogleDrive, "file:///etc/passwd", false},
		{GoogleDrive, "", false},
		// Wrong provider
		{OneDrive, "https://www.googleapis.com/drive/v3/files/abc", false},
		{GoogleDrive, "https://graph.microsoft.com/v1.0/me", false},
	}

	for _, tc := range cases {
		_, err := tc.list.Check(tc.url)
		if (err == nil) != tc.ok {
			t.Errorf("Check(%q) error = %v, want ok=%v", tc.url, err, tc.ok)
		}
	}
}

func TestClient_RedirectsOnlyToAllowedHosts(t *testing.T) {
	client := GoogleDrive.Client(time.Second)
	redirect := func(target string) error {
		u, _ := url.Parse(target)
		return client.CheckRedirect(&http.Request{URL: u}, []*http.Request{{}})
	}

	if err := redirect("https://doc-0s-1k-docs.googleusercontent.com/docs/abc"); err != nil {
		t.Errorf("redirect to Drive content host rejected: %v", err)
	}
	if err := redirect("http://face-service:8081/"); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("redirect to internal host: got %v, want ErrNotAllowed", err)
	}
}

// Even if a URL passed the host check, the client must not connect to a non-public address
// (for example an allowed name that resolves to an internal IP).
func TestClient_RefusesNonPublicAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request reached the loopback server")
	}))
	defer server.Close()

	_, err := OneDrive.Client(time.Second).Get(server.URL)
	if !errors.Is(err, ErrNotAllowed) {
		t.Errorf("got %v, want ErrNotAllowed", err)
	}
}

func TestDenyNonPublic(t *testing.T) {
	for addr, public := range map[string]bool{
		"8.8.8.8:443":            true,
		"[2606:4700::1111]:443":  true,
		"127.0.0.1:443":          false,
		"10.0.1.5:443":           false,
		"172.18.0.3:8081":        false,
		"192.168.1.1:443":        false,
		"169.254.169.254:80":     false,
		"100.64.0.1:443":         false,
		"0.0.0.0:443":            false,
		"[::1]:443":              false,
		"[fd00::1]:443":          false,
		"[fe80::1]:443":          false,
		"[::ffff:127.0.0.1]:443": false,
	} {
		if err := denyNonPublic("tcp", addr, nil); (err == nil) != public {
			t.Errorf("denyNonPublic(%s) = %v, want public=%v", addr, err, public)
		}
	}
}

func TestRedactURL_RemovesPathAndQuery(t *testing.T) {
	cause := &url.Error{Op: "Get", URL: "https://public.am.files.1drv.com/y4m/photo.jpg?tempauth=SECRET-TOKEN", Err: errors.New("i/o timeout")}
	wrapped := fmt.Errorf("failed to execute download request: %w", RedactURL(cause))

	if strings.Contains(wrapped.Error(), "SECRET") || strings.Contains(wrapped.Error(), "y4m") {
		t.Errorf("error text still contains the URL's path or query: %s", wrapped)
	}
	if !strings.Contains(wrapped.Error(), "public.am.files.1drv.com") || !strings.Contains(wrapped.Error(), "i/o timeout") {
		t.Errorf("expected host and cause to remain: %s", wrapped)
	}
	var urlErr *url.Error
	if !errors.As(wrapped, &urlErr) {
		t.Error("the *url.Error should still be inspectable (e.g. for timeouts)")
	}
	if plain := errors.New("x"); RedactURL(plain) != plain {
		t.Error("other errors must pass through unchanged")
	}
}
