package logger

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Middleware returns a fiber middleware for logging requests
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Handle the request and capture errors to set correct status codes (e.g., 404)
		err := c.Next()
		if err != nil {
			if handlerErr := c.App().Config().ErrorHandler(c, err); handlerErr != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		stop := time.Now()

		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		latency := stop.Sub(start)
		ip := c.IP()

		// ANSI colors
		reset := "\033[0m"
		
		statusColor := "\033[32m" // Green
		if status >= 500 {
			statusColor = "\033[31m" // Red
		} else if status >= 400 {
			statusColor = "\033[33m" // Yellow
		} else if status >= 300 {
			statusColor = "\033[34m" // Blue
		}

		methodColor := "\033[36m" // Cyan
		if method == "POST" {
			methodColor = "\033[32m" // Green
		} else if method == "PUT" || method == "PATCH" {
			methodColor = "\033[33m" // Yellow
		} else if method == "DELETE" {
			methodColor = "\033[31m" // Red
		}

		// Very compact format: STATUS|LATENCY|IP|METHOD|PATH
		msg := fmt.Sprintf("%s%d%s|%s|%s|%s%s%s|%s",
			statusColor, status, reset,
			latency,
			ip,
			methodColor, method, reset,
			path,
		)

		Info(msg)
		return nil // Error already handled
	}
}
