package cloud_functions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	errors2 "github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

type ratingStats struct {
	OneStar       int64     `firestore:"oneStar"`
	TwoStar       int64     `firestore:"twoStar"`
	ThreeStar     int64     `firestore:"threeStar"`
	FourStar      int64     `firestore:"fourStar"`
	FiveStar      int64     `firestore:"fiveStar"`
	AverageRating float64   `firestore:"avgRating"`
	TotalRating   float64   `firestore:"totalRating"`
	CreatedDate   time.Time `firestore:"createdDate"`
}

func calculateRating(ctx context.Context, client *firestore.Client) error {
	execFunc := func(req *SettingsRequest) error {
		ratingStatsRef := client.Collection(db.Businesses).Doc(req.BusinessID).
			Collection(db.RatingStats)
		statsSnapshot, _ := ratingStatsRef.Doc(req.YesterdayID).Get(ctx)
		if statsSnapshot != nil && statsSnapshot.Exists() {
			return errors.New(fmt.Sprintf("stats exist for this time ID:%s", req.YesterdayID))
		}
		stats := ratingStats{
			CreatedDate: *req.Yesterday,
		}
		calculateRating := func(rating float64) {
			stats.TotalRating += rating
			if rating <= 1 {
				stats.OneStar += 1
				return
			}
			if rating > 1 && rating <= 2 {
				stats.TwoStar += 1
				return
			}
			if rating > 2 && rating <= 3 {
				stats.ThreeStar += 1
				return
			}
			if rating > 3 && rating <= 4 {
				stats.FourStar += 1
				return
			}
			if rating > 4 && rating <= 5 {
				stats.FiveStar += 1
				return
			}
		}
		chatsIterator := client.Collection(db.TextSessions).
			Where("business.id", "==", req.BusinessID).
			Where("type", "==", 4).
			Documents(ctx)
		for {
			chatSnapshot, err := chatsIterator.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Println("continue with error", err)
				continue
			}
			messagesIter := chatSnapshot.Ref.Collection(db.Messages).
				Where("ratedDate", ">=", req.YesterdayDate).
				Where("ratedDate", "<", req.TodayDate).
				Documents(ctx)
			for {
				messageSnapshot, err := messagesIter.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					log.Println("continue with error", err)
					continue
				}
				data := messageSnapshot.Data()
				if rating, ok := data["rating"].(float64); ok {
					calculateRating(rating)
					continue
				}
				if rating, ok := data["rating"].(int64); ok {
					calculateRating(float64(rating))
				}
			}
		}
		statsSnapshot, err := ratingStatsRef.Doc(req.BeforeYesterdayID).Get(ctx)
		if err == nil && statsSnapshot.Exists() {
			var prevStats *ratingStats
			if err = statsSnapshot.DataTo(&prevStats); err == nil {
				stats.OneStar += prevStats.OneStar
				stats.TwoStar += prevStats.TwoStar
				stats.ThreeStar += prevStats.ThreeStar
				stats.FourStar += prevStats.FourStar
				stats.FiveStar += prevStats.FiveStar
				stats.TotalRating += prevStats.TotalRating
			}
		}
		if stats.TotalRating > 0 {
			divider := float64(stats.OneStar + stats.TwoStar + stats.ThreeStar + stats.FourStar + stats.FiveStar)
			stats.AverageRating = stats.TotalRating / divider
		}
		_, err = ratingStatsRef.Doc(req.YesterdayID).Set(ctx, stats)
		if err != nil {
			msg := fmt.Sprintf("failed to set stats for business:%s, date:%v, err:%v\n", req.BusinessID, req.YesterdayDate, err)
			return errors2.Wrap(err, msg)
		}
		return nil
	}
	return IterateSettings(ctx, client, execFunc)
}
