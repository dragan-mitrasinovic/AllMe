package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// CORSConfig returns CORS middleware configured with domain from environment
func CORSConfig() echo.MiddlewareFunc {
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		// Fallback to localhost for development
		return middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     []string{"http://localhost:4200", "http://localhost:3000"},
			AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
			AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
			AllowCredentials: true,
			MaxAge:           86400, // 24 hours
		})
	}

	// Production CORS configuration - restrict to HTTPS only for production
	allowedOrigins := []string{
		"https://" + domain,
	}

	// Only allow HTTP for explicit non-production domains
	if strings.Contains(domain, "localhost") || strings.Contains(domain, "127.0.0.1") {
		allowedOrigins = append(allowedOrigins, "http://"+domain)
	}

	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	})
}

// SecurityHeaders adds security headers to all responses
func SecurityHeaders() echo.MiddlewareFunc {
	domain := os.Getenv("DOMAIN")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Basic security headers
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "SAMEORIGIN")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Content Security Policy
			// This allows API responses but restricts script execution
			csp := "default-src 'none'; frame-ancestors 'self'"
			if domain != "" && !strings.Contains(domain, "localhost") {
				csp = "default-src 'none'; frame-ancestors https://" + domain
			}
			c.Response().Header().Set("Content-Security-Policy", csp)

			// Permissions Policy - restrict sensitive browser features
			c.Response().Header().Set("Permissions-Policy",
				"geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=()")

			// HSTS - only for HTTPS requests
			// Check if request came through HTTPS (via proxy or direct)
			proto := c.Request().Header.Get("X-Forwarded-Proto")
			if proto == "https" || strings.HasPrefix(c.Request().URL.String(), "https://") {
				// 1 year HSTS with subdomains
				c.Response().Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			return next(c)
		}
	}
}

// SameOriginWrites rejects state-changing requests (anything but GET/HEAD/OPTIONS) whose Origin header
// is not the frontend's origin. Together with the SameSite=Lax session cookie this blocks cross-site
// request forgery. Requests without an Origin header (non-browser clients) carry no session cookie
// from a victim's browser, so they are allowed through. Without FRONTEND_URL the check is off.
func SameOriginWrites() echo.MiddlewareFunc {
	allowed := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}

			origin := c.Request().Header.Get(echo.HeaderOrigin)
			if allowed != "" && origin != "" && origin != allowed {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "cross-origin request rejected",
				})
			}
			return next(c)
		}
	}
}
