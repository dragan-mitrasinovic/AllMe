package main

import (
	"all-me-backend/internal/auth"
	"all-me-backend/internal/download"
	"all-me-backend/internal/face"
	"all-me-backend/internal/middleware"
	"all-me-backend/internal/providers/googledrive"
	"all-me-backend/internal/providers/onedrive"
	"all-me-backend/internal/storage"
	"all-me-backend/internal/thumbnail"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load .env file for local development (ignored in Docker)
	if os.Getenv("DOCKER_ENV") == "" {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}

	e := echo.New()
	initialize(e)

	// Start server
	log.Println("Starting All Me server on :8080")
	log.Fatal(http.ListenAndServe(":8080", e))
}

func initialize(e *echo.Echo) {
	// Initialize provider services
	googleDriveService := googledrive.NewGoogleDriveService()
	oneDriveService := onedrive.NewOneDriveService()

	// Initialize auth service with provider dependencies
	authService := auth.NewService(googleDriveService, oneDriveService)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterRoutes(e)

	// Initialize storage service with provider dependencies
	storageService := storage.NewService(googleDriveService, oneDriveService)
	storageHandler := storage.NewHandler(storageService, authService)
	storageHandler.RegisterRoutes(e)

	// Initialize face service with storage service dependency
	faceService := face.NewService(storageService)
	faceHandler := face.NewHandler(faceService, authService)
	faceHandler.RegisterRoutes(e)

	// Initialize download service with storage service dependency
	downloadService := download.NewService(storageService)
	downloadHandler := download.NewHandler(downloadService, authService)
	downloadHandler.RegisterRoutes(e)

	// Initialize thumbnail proxy handler with provider services
	thumbnailHandler := thumbnail.NewHandler(authService, googleDriveService, oneDriveService)
	thumbnailHandler.RegisterRoutes(e)

	// Middleware
	e.Use(echoMiddleware.Logger())
	e.Use(echoMiddleware.Recover())
	e.Use(middleware.SecurityHeaders())
	e.Use(middleware.CORSConfig())
}
