package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

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

func discord(c echo.Context) error {
	randomKey, err := rdb.RandomKey(ctx).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	protocol := "http"
	if strings.ToUpper(os.Getenv("HTTPS")) == "TRUE" {
		protocol = "https"
	}
	tiktokVid, err := rdb.Get(ctx, randomKey).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	filename := uuid.New().String()
	fo, err := os.Create(fmt.Sprintf("%s.mp4", filename))
	if err != nil {
		panic(err)
	}
	fo.Write(downloadVideo(tiktokVid))
	fo.Close()
	std, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=width,height", "-of", "default=noprint_wrappers=1", fmt.Sprintf("%s.mp4", filename)).Output()
	if err != nil {
		panic(err)
	}
	dimensions := strings.Split(string(std), "\n")
	err = os.Remove(fmt.Sprintf("%s.mp4", filename))
	if err != nil {
		fmt.Println("Failed to cleanup file, could not delete")
	}

	return c.Render(http.StatusOK, "tiktok.html", map[string]interface{}{
		"ogDataVideoSrc":    fmt.Sprintf("%s://%s/api/v1/videos/%s.mp4", protocol, os.Getenv("DOMAIN"), randomKey),
		"ogDataVideoHeight": strings.Replace(dimensions[1], "height=", "", -1),
		"ogDataVideoWidth":  strings.Replace(dimensions[0], "width=", "", -1),
	})
}

// Todo Generate key -> push to url with key, -> use key to return video
func redirect(c echo.Context) error {
	return c.Redirect(http.StatusTemporaryRedirect, "/api/v1/video.mp4")
}

func getVideos(c echo.Context) error {
	validSession := authenticator(c)
	if !validSession {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"videos": nil,
		})
	}
	keys, err := fetchKeys()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"videos": keys,
	})
}

func validateVideo(url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Referer", url)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return errors.New("invalid URL, not returning 200 response or 206 response")
	}
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return nil
}
func createVideo(c echo.Context) error {
	validSession := authenticator(c)
	if !validSession {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"newVideo": nil,
		})
	}
	payload := new(VideoPayload)
	if err := c.Bind(payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	key, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	tiktokVid := payload.VideoURL
	err = validateVideo(tiktokVid)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}
	err = rdb.Set(ctx, key.String(), tiktokVid, 0).Err()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"error": nil,
	})
}

func getRandomVideo(c echo.Context) error {
	randomKey, err := rdb.RandomKey(ctx).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	tiktokVid, err := rdb.Get(ctx, randomKey).Result()
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	// Swap out for inmemory file
	videoContents := downloadVideo(tiktokVid)
	return c.Blob(http.StatusOK, "video/mp4", videoContents)
}

func deleteVideo(c echo.Context) error {
	if !authenticator(c) {
		return c.NoContent(http.StatusUnauthorized)
	}
	id := c.Param("id")
	rdb.Del(ctx, id)
	return c.NoContent(http.StatusOK)
}

func getVideo(c echo.Context) error {
	id := c.Param("id")
	id = strings.Replace(id, ".mp4", "", 1)
	video, err := rdb.Get(ctx, id).Result()
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"video": nil,
		})
	}
	videoContents := downloadVideo(video)
	return c.Blob(http.StatusOK, "video/mp4", videoContents)
}
