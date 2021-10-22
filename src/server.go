package main

import (
	"context"
	crypto_rand "crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"io"
	math_rand "math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var sessions = []string{}
var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDISHOST"),
	Password: "", // no password set
	DB:       0,  // use default DB
})

type AuthenticationPayload struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}
type CustomValidator struct {
	validator *validator.Validate
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func fetchKeys() ([]string, error) {
	keys, err := rdb.Keys(ctx, "*").Result()
	if err != nil {
		panic(err)
	}
	return keys, nil
}

func downloadVideo(url string) []byte {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		// Retry with the cursed host change
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			panic(err)
		}
		req.Header.Set("Referer", url)
		resp, err = client.Do(req)
		if err != nil {
			panic(err)
		}
		if resp.StatusCode != 200 && resp.StatusCode != 206 {
			panic(errors.New("invalid URL, not returning 200 response or 206"))
		}
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return bytes
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		// Optionally, you could return the error to give each route more control over the status code
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

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

func discord(c echo.Context) error {
	randomKey, err := rdb.RandomKey(ctx).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	protocol := "http"
	if strings.ToUpper(os.Getenv("HTTPS")) == "TRUE" {
		protocol = "https"
	}
	return c.Render(http.StatusOK, "tiktok.html", map[string]interface{}{
		"ogDataVideoSrc": fmt.Sprintf("%s://%s/api/v1/videos/%s.mp4", protocol, c.Request().Host, randomKey),
	})
}

// Todo Generate key -> push to url with key, -> use key to return video
func redirect(c echo.Context) error {
	return c.Redirect(http.StatusTemporaryRedirect, "/api/v1/video.mp4")
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

func main() {
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package with cryptographically secure random number generator")
	}
	math_rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
	e := echo.New()
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1 KB
		LogLevel:  log.ERROR,
	}))
	e.Use(middleware.Logger())
	e.Validator = &CustomValidator{validator: validator.New()}
	t := &Template{
		templates: template.Must(template.ParseGlob("static/*.html")),
	}
	e.Renderer = t
	// e.GET("/", getVideo).Name = "Tiktoks"
	e.GET("/", redirect)
	e.GET("/api/v1/videos", getVideos)    // Authenticated
	e.POST("/api/v1/videos", createVideo) // Authenticated
	e.GET("/api/v1/videos/:id", getVideo) // Authenticated TODO: add .mp4 ?
	e.DELETE("/api/v1/videos/:id", deleteVideo)
	e.GET("/api/v1/video.mp4", getRandomVideo)
	e.GET("/api/v1/videos/discord", discord)
	e.GET("/admin", adminSite)
	e.POST("/api/v1/login", login)
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/style.css", "static/style.css")
	e.File("/admin.css", "static/admin.css")
	if rdb != nil {
		if os.Getenv("SERVERPORT") == "443" {
			e.AutoTLSManager.Cache = autocert.DirCache("/var/www/.cache")
			s := http.Server{
				Addr:    ":443",
				Handler: e,
				TLSConfig: &tls.Config{
					GetCertificate: e.AutoTLSManager.GetCertificate,
					NextProtos:     []string{acme.ALPNProto},
				},
			}
			if err = s.ListenAndServeTLS(os.Getenv("CERTFILE"), os.Getenv("KEYFILE")); err != http.ErrServerClosed {
				e.Logger.Fatal(err)
			}
		} else {
			e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("SERVERPORT"))))
		}
	} else {
		e.Logger.Fatal("Failed to connect to redis")
	}
}
