package main

import (
	"log"

	"ai-bridges/internal/server"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"

	_ "ai-bridges/docs" // Import generated docs

	fiberSwagger "github.com/swaggo/fiber-swagger"
)

// @title AI Bridges API
// @version 1.0
// @description WebAI-to-API service for Go - Bridge Web interfaces of AI assistants to standard APIs
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@ai-bridges.dev

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:3000
// @BasePath /api/v1

func main() {
	app := fiber.New()
	app.Use(logger.New())

	// Swagger endpoint
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	server.RegisterRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
