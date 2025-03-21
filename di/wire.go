//go:build wireinject
// +build wireinject

package di

import (
	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/internal/endpoints/textsessions"
	"github.com/VinothKuppanna/pigeon-go/pkg/data"
	"github.com/google/wire"
)

func InitChatsHandler(client *firestore.Client) *textsessions.Handler {
	wire.Build(textsessions.NewHandler, data.NewTextSessionsRepo)
	return &textsessions.Handler{}
}
