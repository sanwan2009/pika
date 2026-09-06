package repo

import (
	"github.com/go-orz/orz"
	"github.com/pika-monitor/pika/internal/models"
	"gorm.io/gorm"
)

type PropertyRepo struct {
	orz.Repository[models.Property, string]
	db *gorm.DB
}

func NewPropertyRepo(db *gorm.DB) *PropertyRepo {
	return &PropertyRepo{
		Repository: orz.NewRepository[models.Property, string](db),
		db:         db,
	}
}
