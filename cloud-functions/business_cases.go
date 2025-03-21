package cloud_functions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
)

type casesStats struct {
	Opened      int64     `firestore:"opened"`
	Closed      int64     `firestore:"closed"`
	Ongoing     int64     `firestore:"ongoing"`
	CreatedDate time.Time `firestore:"createdDate"`
}

func businessCases(ctx context.Context, client *firestore.Client) error {
	execFunc := func(req *SettingsRequest) error {
		casesStatsRef := client.Collection(db.Businesses).Doc(req.BusinessID).Collection(db.CasesStats)
		statsSnapshot, err := casesStatsRef.Doc(req.YesterdayID).Get(ctx)
		if statsSnapshot != nil && statsSnapshot.Exists() {
			return errors.New(fmt.Sprintf("stats exist for this time ID:%s", req.YesterdayID))
		}
		stats := casesStats{
			CreatedDate: *req.Yesterday,
		}
		casesRef := client.Collection(db.Businesses).Doc(req.BusinessID).Collection(db.Cases)
		all, err := casesRef.Where("openedDate", ">=", req.YesterdayDate).
			Where("openedDate", "<", req.TodayDate).
			Select().Documents(ctx).GetAll()
		if err != nil {
			log.Println("opened cases continue with error", err)
		}
		if all != nil {
			stats.Opened = int64(len(all))
		}
		all, err = casesRef.Where("closedDate", ">=", req.YesterdayDate).
			Where("closedDate", "<", req.TodayDate).
			Select().Documents(ctx).GetAll()
		if err != nil {
			log.Println("closed cases continue with error", err)
		}
		if all != nil {
			stats.Closed = int64(len(all))
		}
		statsSnapshot, err = casesStatsRef.Doc(req.BeforeYesterdayID).Get(ctx)
		if err == nil && statsSnapshot.Exists() {
			var prevStats *casesStats
			if err = statsSnapshot.DataTo(&prevStats); err == nil {
				stats.Opened += prevStats.Opened
				stats.Closed += prevStats.Closed
			}
		}
		stats.Ongoing = stats.Opened - stats.Closed
		_, err = casesStatsRef.Doc(req.YesterdayID).Set(ctx, stats)
		if err != nil {
			log.Printf("failed to set stats for business:%s, date:%v, err:%v\n", req.BusinessID, req.Today, err)
			return err
		}
		return nil
	}
	return IterateSettings(ctx, client, execFunc)
}
