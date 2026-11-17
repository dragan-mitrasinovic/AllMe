// Package urlguard restricts which URLs the backend fetches on behalf of users. URLs for thumbnails and
// downloads reach the backend from the browser, so without these checks the server could be made to request
// internal addresses (the face service, cloud metadata endpoints) or send a user's token to other hosts.
package urlguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrNotAllowed is returned for any URL or address the backend must not fetch.
var ErrNotAllowed = errors.New("URL not allowed")

// Allowlist is the set of hosts one storage provider's files and API live on.
type Allowlist struct {
	hosts    []string // matched exactly
	suffixes []string // the domain itself and any subdomain
}

var (
	// GoogleDrive: the Drive API, and the content hosts Drive downloads redirect to.
	GoogleDrive = &Allowlist{
		hosts:    []string{"www.googleapis.com"},
		suffixes: []string{"googleusercontent.com"},
	}
	// OneDrive: Microsoft Graph, and the hosts Graph's pre-authenticated download and thumbnail URLs point to
	// (personal OneDrive, current and legacy; OneDrive for Business / SharePoint; SharePoint's media proxy).
	OneDrive = &Allowlist{
		hosts:    []string{"graph.microsoft.com"},
		suffixes: []string{"microsoftpersonalcontent.com", "1drv.com", "livefilestore.com", "sharepoint.com", "svc.ms"},
	}
)

// ForProvider returns the allowlist for a provider name ("googledrive" or "onedrive").
func ForProvider(provider string) (*Allowlist, error) {
	switch provider {
	case "googledrive":
		return GoogleDrive, nil
	case "onedrive":
		return OneDrive, nil
	default:
		return nil, fmt.Errorf("%w: unsupported provider %q", ErrNotAllowed, provider)
	}
}

// Check parses raw and returns it if it is an https URL on an allowed host, without credentials,
// IP literals or non-standard ports.
func (a *Allowlist) Check(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil {
		return nil, ErrNotAllowed
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, ErrNotAllowed
	}

	host := strings.ToLower(u.Hostname())
	if host == "" || net.ParseIP(host) != nil || !a.allowsHost(host) {
		return nil, ErrNotAllowed
	}
	return u, nil
}

// IsHost reports whether u's host is exactly host (case-insensitive).
func IsHost(u *url.URL, host string) bool {
	return strings.EqualFold(u.Hostname(), host)
}

func (a *Allowlist) allowsHost(host string) bool {
	for _, h := range a.hosts {
		if host == h {
			return true
		}
	}
	for _, s := range a.suffixes {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}

// Client returns an HTTP client that only follows redirects to hosts on the allowlist and refuses to
// connect to loopback, private, link-local and other non-public addresses, whatever DNS returns.
func (a *Allowlist) Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: denyNonPublic}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext

	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: a.checkRedirect,
	}
}

func (a *Allowlist) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if _, err := a.Check(req.URL.String()); err != nil {
		return fmt.Errorf("redirect to %s: %w", req.URL.Hostname(), err)
	}
	return nil
}

// denyNonPublic runs after DNS resolution, on the actual address being dialed.
func denyNonPublic(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrNotAllowed
	}
	ip := net.ParseIP(host)
	if ip == nil || !isPublic(ip) {
		return fmt.Errorf("%w: address %s", ErrNotAllowed, host)
	}
	return nil
}

var sharedAddressSpace = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isPublic(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		sharedAddressSpace.Contains(ip))
}

// RedactURL returns err with the request URL of any *url.Error cut down to scheme and host. Download and
// thumbnail URLs can carry credentials (OneDrive's pre-authenticated URLs include a tempauth token), and
// net/http puts the full URL into its errors, which end up in logs.
func RedactURL(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	redacted := *urlErr
	if u, parseErr := url.Parse(urlErr.URL); parseErr == nil && u.Host != "" {
		redacted.URL = u.Scheme + "://" + u.Host
	} else {
		redacted.URL = "<redacted>"
	}
	return &redacted
}
