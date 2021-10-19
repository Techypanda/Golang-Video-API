package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDISHOST"),
	Password: "", // no password set
	DB:       0,  // use default DB
})

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func fetchKeys() ([]string, error) {
	var keys []string
	var cursor uint64
	for {
		var err error
		keys, cursor, err = rdb.Scan(ctx, cursor, "*", 0).Result()
		if err != nil {
			return keys, err
		}
		if cursor == 0 { // no more keys
			break
		}
	}
	return keys, nil
}

func getDepression(c echo.Context) error {
	keys, err := fetchKeys()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	tiktokVid, err := rdb.Get(ctx, keys[rand.Intn(len(keys))]).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.Render(http.StatusOK, "tiktok.html", template.HTML(tiktokVid))
}

func main() {
	rand.Seed(time.Now().Unix())
	e := echo.New()
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1 KB
		LogLevel:  log.ERROR,
	}))
	e.Use(middleware.Logger())
	t := &Template{
		templates: template.Must(template.ParseGlob("static/*.html")),
	}
	e.Renderer = t
	e.GET("/", getDepression).Name = "depressing tiktok"
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/style.css", "static/style.css")
	if rdb != nil {
		e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("SERVERPORT"))))
	} else {
		e.Logger.Fatal("Failed to connect to redis")
	}
}
