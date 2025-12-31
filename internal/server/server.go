package server

import (
	"log"

	geminiHandlers "ai-bridges/internal/handlers/gemini"
	openaiHandlers "ai-bridges/internal/handlers/openai"
	geminiProvider "ai-bridges/internal/providers/gemini"
	"ai-bridges/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

// Server represents the API server
type Server struct {
	app          *fiber.App
	geminiClient *geminiProvider.Client
}

// New creates a new server instance
func New(geminiClient *geminiProvider.Client) (*Server, error) {
	app := fiber.New(fiber.Config{
		AppName: "AI Bridges API",
	})

	// Add global middlewares
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS, PATCH",
	}))
	app.Use(logger.Middleware())
	app.Use(recover.New())

	server := &Server{
		app:          app,
		geminiClient: geminiClient,
	}

	// Register routes
	server.registerRoutes()

	return server, nil
}

// registerRoutes registers all API routes
func (s *Server) registerRoutes() {
	// Swagger documentation
	s.app.Get("/swagger/*", fiberSwagger.WrapHandler)

	// Register provider-specific routes at root level
	geminiHandlers.RegisterRoutes(s.app, s.geminiClient)
	openaiHandlers.RegisterRoutes(s.app, s.geminiClient)

	// Health check
	s.app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "ai-bridges",
		})
	})
}

// App returns the fiber app instance
func (s *Server) App() *fiber.App {
	return s.app
}

// Start starts the server
func (s *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)
	return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	log.Println("Shutting down server...")
	return s.app.Shutdown()
}
