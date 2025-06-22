package auth

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User represents a user in the system
type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty" example:"683cdb8aa96ad71e8e075bd1"`
	Email        string        `bson:"email" json:"email" example:"test@example.com"`
	PasswordHash string        `bson:"password_hash" json:"-" example:"$2a$10$1234567890"`
	CreatedAt    time.Time     `bson:"created_at" json:"created_at" example:"2025-06-01T23:00:26.005703677Z"`
	UpdatedAt    time.Time     `bson:"updated_at" json:"updated_at" example:"2025-06-01T23:00:26.005703677Z"`
}
