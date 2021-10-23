package main

import (
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func login(c echo.Context) error {
	auth := new(AuthenticationPayload)
	if err := c.Bind(auth); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(auth); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if auth.Username == "admin" && auth.Password == os.Getenv("ADMINPASSWORD") {
		guid, err := uuid.NewRandom()
		if err != nil {
			panic(err)
		}
		sessions = append(sessions, guid.String())
		cookie := new(http.Cookie)
		cookie.Name = "session"
		cookie.Value = guid.String()
		cookie.Expires = time.Now().Add(24 * time.Hour)
		cookie.HttpOnly = true
		cookie.Path = "/"
		c.SetCookie(cookie)
		return c.JSON(http.StatusOK, auth)
	} else {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{"error": "Bad Password/Username"})
	}
}

func authenticator(c echo.Context) bool {
	cookie, err := c.Cookie("session")
	if err != nil {
		return false
	}
	validSession := false
	for _, session := range sessions {
		if cookie.Value == session {
			validSession = true
			break
		}
	}
	return validSession
}

func adminSite(c echo.Context) error {
	validSession := authenticator(c)
	if validSession {
		return c.Render(http.StatusOK, "admin.html", map[string]interface{}{
			"admin": "true",
		})
	} else {
		cookie := new(http.Cookie)
		cookie.Name = "session"
		cookie.MaxAge = -1
		cookie.Path = "/"
		c.SetCookie(cookie) // Clear Cookie
		return c.Render(http.StatusOK, "admin.html", map[string]interface{}{
			"admin": "false",
		})
	}
}
