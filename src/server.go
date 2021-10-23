package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	math_rand "math/rand"
	"os"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

var sessions = []string{}
var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDISHOST"),
	Password: "", // no password set
	DB:       0,  // use default DB
})

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func makeRoute(e *echo.Route, description string) {
	e.Name = description
	fmt.Printf("→ %s ☆ %s ♡ %s\n", e.Path, e.Name, e.Method)
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
	e.Use(middleware.Gzip())
	e.Validator = &CustomValidator{validator: validator.New()}
	t := &Template{
		templates: template.Must(template.ParseGlob("static/*.html")),
	}
	e.Renderer = t

	makeRoute(e.GET("/", redirect), "Redirect To /api/v1/video.mp4")
	makeRoute(e.GET("/api/v1/videos", getVideos), "Get All Videos (Authenticated)")
	makeRoute(e.POST("/api/v1/videos", createVideo), "Create Video (Authenticated)")
	makeRoute(e.GET("/api/v1/videos/:id", getVideo), "Get Specific Video (.mp4 is ignored)")
	makeRoute(e.DELETE("/api/v1/videos/:id", deleteVideo), "Remove Video (Authenticated)")
	makeRoute(e.GET("/api/v1/video.mp4", getRandomVideo), "Get random video")
	makeRoute(e.GET("/api/v1/videos/discord", discord), "Return a html page containing OG Data for discord")
	makeRoute(e.GET("/admin", adminSite), "Return admin page")
	makeRoute(e.POST("/api/v1/login", login), "Login endpoint attach session")
	makeRoute(e.File("/favicon.ico", "static/favicon.ico"), "Favicon endpoint")
	makeRoute(e.File("/style.css", "static/style.css"), "CSS endpoint")
	makeRoute(e.File("/admin.css", "static/admin.css"), "Admin CSS Endpoint")

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("PORT"))))
}
