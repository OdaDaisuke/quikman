package model

import (
	"time"
)

type AccountModel struct {
	ID        uint64
	Email     string
	Name      string
	CreatedAt time.Time
}
