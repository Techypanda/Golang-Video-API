package main

import (
	"errors"
	"io"
	math_rand "math/rand"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type VideoPayload struct {
	VideoURL string `json:"videoURL" validate:"required"`
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
	video, err := rdb.Get(ctx, id).Result()
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"video": nil,
		})
	}
	videoContents := downloadVideo(video)
	return c.Blob(http.StatusOK, "video/mp4", videoContents)
}
