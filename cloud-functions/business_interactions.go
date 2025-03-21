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

type interactionsStats struct {
	Weekday         int       `firestore:"weekday"`
	OpenedCases     int64     `firestore:"openedCases"`
	OpenedCasesWeek int64     `firestore:"openedCasesWeek"`
	NewAppoints     int64     `firestore:"newAppoints"`
	NewAppointsWeek int64     `firestore:"newAppointsWeek"`
	DiffCases       int64     `firestore:"diffCases"`
	DiffAppoints    int64     `firestore:"diffAppoints"`
	CreatedDate     time.Time `firestore:"createdDate"`
}

func interactions(ctx context.Context, client *firestore.Client) error {
	execFunc := func(req *SettingsRequest) error {
		businessRef := client.Collection(db.Businesses).Doc(req.BusinessID)
		interactionsStatsRef := businessRef.Collection(db.InteractionsStats)
		statsSnapshot, err := interactionsStatsRef.Doc(req.YesterdayID).Get(ctx)
		if statsSnapshot != nil && statsSnapshot.Exists() {
			return errors.New(fmt.Sprintf("stats exist for this time ID:%s", req.YesterdayID))
		}
		stats := interactionsStats{
			Weekday:     int(req.Yesterday.Weekday()),
			CreatedDate: *req.Yesterday,
		}
		casesRef := businessRef.Collection(db.Cases)
		all, err := casesRef.Where("openedDate", ">=", req.YesterdayDate).
			Where("openedDate", "<", req.TodayDate).
			Select().Documents(ctx).GetAll()
		if err != nil {
			log.Println("opened cases continue with error", err)
		}
		if all != nil {
			stats.OpenedCases = int64(len(all))
		}
		appointsRef := businessRef.Collection(db.Appoints)
		all, err = appointsRef.Where("conducted", "==", true).
			Where("startDate", ">=", req.YesterdayDate).
			Where("startDate", "<", req.TodayDate).
			Select().Documents(ctx).GetAll()
		if err != nil {
			log.Println("new appoints continue with error", err)
		}
		if all != nil {
			stats.NewAppoints = int64(len(all))
		}
		if stats.Weekday > 0 { // we have a new week
			statsSnapshot, err := interactionsStatsRef.Doc(req.BeforeYesterdayID).Get(ctx)
			if err == nil && statsSnapshot.Exists() {
				var prevDayStats *interactionsStats
				if err = statsSnapshot.DataTo(&prevDayStats); err == nil {
					stats.NewAppointsWeek = stats.NewAppoints + prevDayStats.NewAppointsWeek
					stats.OpenedCasesWeek = stats.OpenedCases + prevDayStats.OpenedCasesWeek
				}
			}
		}
		statsSnapshot, err = interactionsStatsRef.Doc(req.WeekBeforeYesterdayID).Get(ctx)
		if err == nil && statsSnapshot.Exists() {
			var prevWeekDayStats *interactionsStats
			if err = statsSnapshot.DataTo(&prevWeekDayStats); err == nil {
				if stats.OpenedCases == 0 && prevWeekDayStats.OpenedCases == 0 {
					stats.DiffCases = 0
				} else if prevWeekDayStats.OpenedCases > 0 {
					stats.DiffCases = (stats.OpenedCases - prevWeekDayStats.OpenedCases) * 100 / prevWeekDayStats.OpenedCases
				} else {
					stats.DiffCases = 100
				}
				if stats.NewAppoints == 0 && prevWeekDayStats.NewAppoints == 0 {
					stats.DiffAppoints = 0
				} else if prevWeekDayStats.NewAppoints > 0 {
					stats.DiffAppoints = (stats.NewAppoints - prevWeekDayStats.NewAppoints) * 100 / prevWeekDayStats.NewAppoints
				} else {
					stats.DiffAppoints = 100
				}
			}
		}
		_, err = interactionsStatsRef.Doc(req.YesterdayID).Set(ctx, stats)
		if err != nil {
			log.Printf("failed to set stats for business:%s, date:%v, err:%v\n", req.BusinessID, req.YesterdayDate, err)
		}
		return nil
	}

	return IterateSettings(ctx, client, execFunc)
}
