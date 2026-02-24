package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"duitno/internal/database"
	"duitno/internal/handlers"
	"duitno/internal/oauth"
)

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func main() {
	projectRoot := findProjectRoot()
	if projectRoot != "" {
		godotenv.Load(filepath.Join(projectRoot, ".env"))
	}

	if err := database.EnsureDatabase(); err != nil {
		log.Fatalf("Failed to ensure database: %v", err)
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db.DB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	oauthConfigs := oauth.LoadOAuthConfigs()
	oauthHandler := handlers.NewOAuthHandler(oauthConfigs, db)

	app := SetupRouter(db, oauthHandler)

	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8080"
	}

	go func() {
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := app.Shutdown(); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited properly")
}

func SetupRouter(db *database.DB, oauthHandler *handlers.OAuthHandler) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "duitno",
	})

	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	app.Get("/health/db", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return c.Status(503).JSON(fiber.Map{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
		}
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	auth := app.Group("/auth")
	auth.Get("/:provider", oauthHandler.GetAuthURL)
	auth.Get("/:provider/callback", oauthHandler.Callback)

	return app
}
