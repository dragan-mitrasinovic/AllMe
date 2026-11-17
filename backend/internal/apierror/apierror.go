// Package apierror sends errors to clients without leaking internals: the detailed error is logged
// server-side and the client only gets a fixed, user-facing message.
package apierror

import "github.com/labstack/echo/v4"

// Respond logs err (if any) with the route and replies with {"error": public}.
func Respond(c echo.Context, status int, public string, err error) error {
	if err != nil {
		// The route pattern, not the URL: query strings can carry share links and file URLs.
		c.Logger().Errorf("%s %s: %v", c.Request().Method, c.Path(), err)
	}
	return c.JSON(status, map[string]string{"error": public})
}

// NotSignedIn is the reply when the session has no token for the requested provider.
func NotSignedIn(c echo.Context, err error) error {
	return Respond(c, 401, "You're not signed in to this cloud storage any more. Please connect again.", err)
}
