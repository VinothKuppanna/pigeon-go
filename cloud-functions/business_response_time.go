package cloud_functions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
	errors2 "github.com/pkg/errors"
)

type responseTimeStats struct {
	Weekday          int       `firestore:"weekday"`
	AcceptTime       int64     `firestore:"acceptTime"`
	DiffAcceptTime   int64     `firestore:"diffAcceptTime"`
	ResponseTime     int64     `firestore:"responseTime"`
	DiffResponseTime int64     `firestore:"diffResponseTime"`
	RejectTime       int64     `firestore:"rejectTime"`
	DiffRejectTime   int64     `firestore:"diffRejectTime"`
	ForwardTime      int64     `firestore:"forwardTime"`
	DiffForwardTime  int64     `firestore:"diffForwardTime"`
	ResolveTime      int64     `firestore:"resolveTime"`
	DiffResolveTime  int64     `firestore:"diffResolveTime"`
	CreatedDate      time.Time `firestore:"createdDate"`
}

func responseTime(ctx context.Context, client *firestore.Client) error {
	execFunc := func(req *SettingsRequest) error {
		businessRef := client.Collection(db.Businesses).Doc(req.BusinessID)
		responseStatsRef := businessRef.Collection(db.ResponseTimeStats)
		statsSnapshot, err := responseStatsRef.Doc(req.YesterdayID).Get(ctx)
		if statsSnapshot != nil && statsSnapshot.Exists() {
			return errors.New(fmt.Sprintf("stats exist for this time ID:%s", req.YesterdayID))
		}
		stats := responseTimeStats{
			Weekday:     int(req.Yesterday.Weekday()),
			CreatedDate: *req.Yesterday,
		}
		acceptTimes := make([]int64, 0)
		rejectTimes := make([]int64, 0)
		forwardTimes := make([]int64, 0)
		resolveTimes := make([]int64, 0)
		responseTimes := make([]int64, 0)

		messagesRef := client.CollectionGroup(db.Messages)
		casesRef := businessRef.Collection(db.Cases)
		all, err := casesRef.Where("acceptedDate", ">=", req.YesterdayDate).
			Where("acceptedDate", "<", req.TodayDate).
			Select("openedDate", "acceptedDate", "associate").Documents(ctx).GetAll()
		if err != nil {
			return err
		}
		for _, doc := range all {
			var bizCase *model.Case
			err = doc.DataTo(&bizCase)
			if err != nil {
				log.Println("continue with err.", err)
				continue
			}
			openedDate := bizCase.OpenedDate
			acceptedDate := bizCase.AcceptedDate
			if openedDate != nil && acceptedDate != nil {
				duration := acceptedDate.Sub(*openedDate) / time.Millisecond
				acceptTimes = append(acceptTimes, int64(duration))

				contactSnapshot, err := businessRef.Collection(db.Directory).Doc(bizCase.Associate.Id).Get(ctx)
				if err != nil || !contactSnapshot.Exists() {
					log.Println("continue with err.", err)
					continue
				}
				var contact *model.Contact
				if err = contactSnapshot.DataTo(&contact); err != nil {
					log.Println("continue with err.", err)
					continue
				}
				if contact.Associate != nil {
					respondedMessages, err := messagesRef.Where("textSessionId", "==", bizCase.TextSessionId).
						Where("sender.uid", "==", contact.Associate.Id).
						Where("createdDate", ">=", acceptedDate).
						OrderBy("createdDate", firestore.Asc).
						Select("createdDate").
						Limit(1).Documents(ctx).GetAll()

					if err == nil && len(respondedMessages) > 0 {
						createdDate := respondedMessages[0].Data()["createdDate"].(time.Time)
						duration := createdDate.Sub(*openedDate) / time.Millisecond
						responseTimes = append(responseTimes, int64(duration))
					}
				}
			}
		}

		all, err = casesRef.Where("rejectedDate", ">=", req.YesterdayDate).
			Where("rejectedDate", "<", req.TodayDate).
			Select("openedDate", "rejectedDate").Documents(ctx).GetAll()
		if err != nil {
			return err
		}
		for _, doc := range all {
			var bizCase *model.Case
			err = doc.DataTo(&bizCase)
			if err != nil {
				log.Println("continue with err.", err)
				continue
			}
			openedDate := bizCase.OpenedDate
			rejectedDate := bizCase.RejectedDate
			if openedDate != nil && rejectedDate != nil {
				duration := rejectedDate.Sub(*openedDate) / time.Millisecond
				rejectTimes = append(rejectTimes, int64(duration))
			}
		}

		all, err = casesRef.Where("forwardedDate", ">=", req.YesterdayDate).
			Where("forwardedDate", "<", req.TodayDate).
			Select("openedDate", "forwardedDate").Documents(ctx).GetAll()
		if err != nil {
			log.Println("continue with err.", err)
			return err
		}
		for _, doc := range all {
			var bizCase *model.Case
			err = doc.DataTo(&bizCase)
			if err != nil {
				log.Println("continue with err.", err)
				continue
			}
			openedDate := bizCase.OpenedDate
			forwardedDate := bizCase.ForwardedDate
			if openedDate != nil && forwardedDate != nil {
				duration := forwardedDate.Sub(*openedDate) / time.Millisecond
				forwardTimes = append(forwardTimes, int64(duration))
			}
		}

		all, err = casesRef.Where("closedDate", ">=", req.YesterdayDate).
			Where("closedDate", "<", req.TodayDate).
			Select("openedDate", "closedDate").Documents(ctx).GetAll()
		if err != nil {
			log.Println("continue with err.", err)
			return err
		}
		for _, doc := range all {
			var bizCase *model.Case
			err = doc.DataTo(&bizCase)
			if err != nil {
				log.Println("continue with err.", err)
				continue
			}
			openedDate := bizCase.OpenedDate
			closedDate := bizCase.ClosedDate
			if openedDate != nil && closedDate != nil {
				duration := closedDate.Sub(*openedDate) / time.Millisecond
				resolveTimes = append(resolveTimes, int64(duration))
			}
		}

		stats.AcceptTime = CalculateAverage(acceptTimes)
		stats.ResponseTime = CalculateAverage(responseTimes)
		stats.RejectTime = CalculateAverage(rejectTimes)
		stats.ForwardTime = CalculateAverage(forwardTimes)
		stats.ResolveTime = CalculateAverage(resolveTimes)

		statsSnapshot, err = responseStatsRef.Doc(req.WeekBeforeYesterdayID).Get(ctx)
		if err == nil && statsSnapshot.Exists() {
			var prevStats *responseTimeStats
			if err = statsSnapshot.DataTo(&prevStats); err == nil {
				if stats.AcceptTime == 0 && prevStats.AcceptTime == 0 {
					stats.DiffAcceptTime = 0
				} else if prevStats.AcceptTime > 0 {
					stats.DiffAcceptTime = (stats.AcceptTime - prevStats.AcceptTime) * 100 / prevStats.AcceptTime
				} else {
					stats.DiffAcceptTime = 100
				}
				if stats.RejectTime == 0 && prevStats.RejectTime == 0 {
					stats.DiffRejectTime = 0
				} else if prevStats.RejectTime > 0 {
					stats.DiffRejectTime = (stats.RejectTime - prevStats.RejectTime) * 100 / prevStats.RejectTime
				} else {
					stats.DiffRejectTime = 100
				}
				if stats.ForwardTime == 0 && prevStats.ForwardTime == 0 {
					stats.DiffForwardTime = 0
				} else if prevStats.ForwardTime > 0 {
					stats.DiffForwardTime = (stats.ForwardTime - prevStats.ForwardTime) * 100 / prevStats.ForwardTime
				} else {
					stats.DiffForwardTime = 100
				}
				if stats.ResolveTime == 0 && prevStats.ResolveTime == 0 {
					stats.DiffResolveTime = 0
				} else if prevStats.ResolveTime > 0 {
					stats.DiffResolveTime = (stats.ResolveTime - prevStats.ResolveTime) * 100 / prevStats.ResolveTime
				} else {
					stats.DiffResolveTime = 100
				}
				if stats.ResponseTime == 0 && prevStats.ResponseTime == 0 {
					stats.DiffResponseTime = 0
				} else if prevStats.ResponseTime > 0 {
					stats.DiffResponseTime = (stats.ResponseTime - prevStats.ResponseTime) * 100 / prevStats.ResponseTime
				} else {
					stats.DiffResponseTime = 100
				}
			}
		}
		_, err = responseStatsRef.Doc(req.YesterdayID).Set(ctx, stats)
		if err != nil {
			msg := fmt.Sprintf("failed to set stats for business:%s, date:%v, err:%v\n", req.BusinessID, req.YesterdayDate, err)
			return errors2.Wrap(err, msg)
		}
		return nil
	}
	return IterateSettings(ctx, client, execFunc)
}
