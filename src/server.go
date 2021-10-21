package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"io"
	math_rand "math/rand"
	"net/http"
	"os"

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

func getVideo(c echo.Context) error {
	keys, err := fetchKeys()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	tiktokVid, err := rdb.Get(ctx, keys[math_rand.Intn(len(keys))]).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	// Swap out for inmemory file
	videoContents := downloadVideo(tiktokVid)
	return c.Blob(http.StatusOK, "video/mp4", videoContents)
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
	t := &Template{
		templates: template.Must(template.ParseGlob("static/*.html")),
	}
	e.Renderer = t
	e.GET("/", getVideo).Name = "Tiktoks"
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/style.css", "static/style.css")
	if rdb != nil {
		e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", os.Getenv("SERVERPORT"))))
	} else {
		e.Logger.Fatal("Failed to connect to redis")
	}
}
