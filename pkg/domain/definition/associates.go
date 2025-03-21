package definition

import (
	"context"

	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
)

type AssociatesRepository interface {
	Delete(ctx context.Context, associateID string) (*model.Associate, error)
}
