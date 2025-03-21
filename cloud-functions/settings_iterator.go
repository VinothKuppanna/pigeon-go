package cloud_functions

import (
	"context"
	"log"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
	"google.golang.org/api/iterator"
)

type SettingsRequest struct {
	BusinessID            string
	YesterdayID           string
	BeforeYesterdayID     string
	WeekBeforeYesterdayID string
	Today                 *time.Time
	TodayDate             *time.Time
	Yesterday             *time.Time
	YesterdayDate         *time.Time
	BeforeYesterday       *time.Time
	BeforeYesterdayDate   *time.Time
}

func IterateSettings(ctx context.Context, client *firestore.Client, body func(*SettingsRequest) error) error {
	documents := client.Collection(db.Settings).Documents(ctx)
	defer documents.Stop()
	for {
		snapshot, err := documents.Next()
		if err == iterator.Done {
			log.Println("done processing")
			break
		}
		if err != nil {
			log.Println("continue with error", err)
			continue
		}
		if !snapshot.Exists() {
			continue
		}
		var settings *model.Settings
		err = snapshot.DataTo(&settings)
		if err != nil {
			log.Println("continue with settings data error", err)
			continue
		}
		timeZone := settings.TimeZone
		if len(timeZone) == 0 {
			timeZone = "+00:00"
		}
		businessId := snapshot.Ref.ID
		now := NowWithOffset(timeZone)
		startDay := now.AddDate(0, -5, 0)
		for today := startDay; today.After(*now) == false; today = today.AddDate(0, 0, 1) {
			yesterday := today.AddDate(0, 0, -1)
			todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
			yesterdayDate := todayDate.AddDate(0, 0, -1)
			beforeYesterdayDate := yesterdayDate.AddDate(0, 0, -1)
			yesterdayUTC := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
			yesterdayID := strconv.FormatInt(yesterdayUTC.Unix(), 10)
			beforeYesterdayUTC := yesterdayUTC.AddDate(0, 0, -1)
			beforeYesterdayID := strconv.FormatInt(beforeYesterdayUTC.Unix(), 10)
			weekBeforeYesterdayUTC := yesterdayUTC.AddDate(0, 0, -7)
			weekBeforeYesterdayID := strconv.FormatInt(weekBeforeYesterdayUTC.Unix(), 10)
			log.Printf("business:%s, tz:%s, time:%v, yesterday:%v, time ID:%s\n", businessId, timeZone, today, yesterdayDate, yesterdayID)

			err = body(&SettingsRequest{
				BusinessID:            businessId,
				YesterdayID:           yesterdayID,
				BeforeYesterdayID:     beforeYesterdayID,
				WeekBeforeYesterdayID: weekBeforeYesterdayID,
				Today:                 &today,
				TodayDate:             &todayDate,
				Yesterday:             &yesterday,
				YesterdayDate:         &yesterdayDate,
				BeforeYesterdayDate:   &beforeYesterdayDate,
			})
			if err != nil {
				log.Println("continue with error", err)
				continue
			}
			log.Printf("stats set for business:%s, date:%v\n", businessId, yesterday)
		}
	}
	return nil
}
