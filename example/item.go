package model

import (
	"time"
)

type ItemModel struct {
	ID          uint64
	Name        string
	Description string
	CreatedAt   time.Time
}
