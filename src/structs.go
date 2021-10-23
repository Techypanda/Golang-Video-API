package main

import (
	"net/http"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type AuthenticationPayload struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}
type VideoPayload struct {
	VideoURL string `json:"videoURL" validate:"required"`
}
type CustomValidator struct {
	validator *validator.Validate
}
type Template struct {
	templates *template.Template
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		// Optionally, you could return the error to give each route more control over the status code
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}
