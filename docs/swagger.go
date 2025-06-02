// Package docs NotePulse API
//
// @title  NotePulse API
// @version 0.1.0
// @description Notes CRUD and live updates.
// @host      localhost:8080
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
package docs

import (
	_ "note-pulse/internal/handlers/httperr"
	_ "note-pulse/internal/services/auth"
	_ "note-pulse/internal/services/notes"
)
