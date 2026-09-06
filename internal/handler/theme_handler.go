package handler

import (
	"errors"
	"net/http"

	"github.com/go-orz/orz"
	"github.com/labstack/echo/v5"
	"github.com/pika-monitor/pika/internal/service"
	"go.uber.org/zap"
)

type ThemeHandler struct {
	logger  *zap.Logger
	service *service.ThemeService
}

func NewThemeHandler(logger *zap.Logger, themeService *service.ThemeService) *ThemeHandler {
	return &ThemeHandler{logger: logger, service: themeService}
}

func (h *ThemeHandler) List(c *echo.Context) error {
	items, err := h.service.List(c.Request().Context())
	if err != nil {
		return err
	}
	return orz.Ok(c, items)
}

func (h *ThemeHandler) Upload(c *echo.Context) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "请选择主题 ZIP 文件")
	}
	if fileHeader.Size <= 0 || fileHeader.Size > 64<<20 {
		return echo.NewHTTPError(http.StatusBadRequest, "主题 ZIP 为空或超过 64 MiB")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	result, err := h.service.Install(c.Request().Context(), file)
	if err != nil {
		return themeHTTPError(err)
	}
	return orz.Ok(c, result)
}

func (h *ThemeHandler) Activate(c *echo.Context) error {
	if err := h.service.Activate(c.Request().Context(), c.Param("id")); err != nil {
		return themeHTTPError(err)
	}
	return orz.Ok(c, orz.Map{})
}

func (h *ThemeHandler) Delete(c *echo.Context) error {
	if err := h.service.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return themeHTTPError(err)
	}
	return orz.Ok(c, orz.Map{})
}

func (h *ThemeHandler) Preview(c *echo.Context) error {
	path, contentType, err := h.service.PreviewPath(c.Request().Context(), c.Param("id"))
	if err != nil {
		return themeHTTPError(err)
	}
	if contentType != "" {
		c.Response().Header().Set("Content-Type", contentType)
	}
	c.Response().Header().Set("Cache-Control", "private, max-age=300")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return serveResolvedStaticFile(c, path, false)
}

func themeHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrThemeNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrThemeExists), errors.Is(err, service.ErrThemeActive):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrDefaultTheme):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
}
