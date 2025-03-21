package cloud_functions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
	errors2 "github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

type staffPerf struct {
	Weekday     int       `firestore:"weekday"`
	CreatedDate time.Time `firestore:"createdDate"`
}

type staffPerfItem struct {
	AssociateId  string  `firestore:"uid"`
	Name         string  `firestore:"name"`
	PhotoUrl     string  `firestore:"photoUrl,omitempty"`
	Activity     int64   `firestore:"activity"`
	ClosedCases  int     `firestore:"closedCases"`
	Rating       float64 `firestore:"rating,omitempty"`
	AcceptTime   int64   `firestore:"acceptTime"`
	ResponseTime int64   `firestore:"responseTime"`
	ResolveTime  int64   `firestore:"resolveTime"`
}

func (spi *staffPerfItem) name(name string) (item *staffPerfItem) {
	spi.Name = name
	item = spi
	return
}

func (spi *staffPerfItem) photoUrl(url string) (item *staffPerfItem) {
	spi.PhotoUrl = url
	item = spi
	return
}

func (spi *staffPerfItem) rating(rating float64) (item *staffPerfItem) {
	spi.Rating = rating
	item = spi
	return
}

func staffPerformance(ctx context.Context, client *firestore.Client) error {
	execFunc := func(req *SettingsRequest) error {
		messagesRef := client.CollectionGroup(db.Messages)
		businessRef := client.Collection(db.Businesses).Doc(req.BusinessID)
		staffPerformanceRef := businessRef.Collection(db.StaffPerformance)
		yesterdayStatsRef := staffPerformanceRef.Doc(req.YesterdayID)
		statsSnapshot, err := yesterdayStatsRef.Get(ctx)
		if statsSnapshot != nil && statsSnapshot.Exists() {
			return errors.New(fmt.Sprintf("next update in: %s", req.TodayDate.Add(24*time.Hour).Sub(*req.Today).String()))
		}
		stats := staffPerf{
			Weekday:     int(req.Yesterday.Weekday()),
			CreatedDate: *req.Yesterday,
		}
		casesRef := businessRef.Collection(db.Cases)
		acceptedCasesToDate, err := casesRef.Where("acceptedDate", ">=", req.YesterdayDate).
			Where("acceptedDate", "<", req.TodayDate).
			OrderBy("acceptedDate", firestore.Asc).
			Select("associate", "openedDate", "acceptedDate", "closedDate", "closedBy", "textSessionId").
			Documents(ctx).GetAll()
		if err != nil {
			return err
		}
		var usersPerfs []*staffPerfItem
		usersIterator := client.Collection(db.Users).Where("business.id", "==", req.BusinessID).Documents(ctx)
		for {
			userDoc, err := usersIterator.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				continue
			}
			var associate *model.Associate
			err = userDoc.DataTo(&associate)
			if err != nil {
				continue
			}
			associate.Id = userDoc.Ref.ID
			userPerf := staffPerfItem{
				AssociateId: associate.Id,
				Name:        associate.FullName,
			}
			if associate.HasPhoto() {
				userPerf.photoUrl(associate.PhotoUrl.Normal)
			}
			if associate.HasStats() {
				userPerf.rating(associate.Stats.AvgRating) // todo: rating must be calculated based on closed message rating
			}
			if associate.WorkingStatus != nil && associate.WorkingStatus.Time != nil {
				weekdayToday := req.Today.Weekday()
				sunday := req.Today.AddDate(0, 0, -int(weekdayToday))
				var workingLogQuery firestore.Query
				if weekdayToday == sunday.Weekday() {
					workingLogQuery = userDoc.Ref.Collection(db.WorkingStatusLog).
						Where("time", ">=", sunday).
						OrderBy("time", firestore.Asc).
						Limit(1)
				} else {
					workingLogQuery = userDoc.Ref.Collection(db.WorkingStatusLog).
						Where("time", ">=", sunday).
						Where("time", "<", req.Today).
						OrderBy("time", firestore.Asc)
				}
				snapshots, err := workingLogQuery.Documents(ctx).GetAll()
				if err != nil {
					continue
				}
				wslog := make([]*model.WorkingStatus, 0)
				for _, doc := range snapshots {
					var ws *model.WorkingStatus
					err := doc.DataTo(&ws)
					if err != nil {
						continue
					}
					wslog = append(wslog, ws)
				}
				wslog = append(wslog, associate.WorkingStatus)
				userPerf.Activity = CalculateStuffActivity(wslog, sunday, req.Today)
			}

			acceptTimes := make([]int64, 0)
			responseTimes := make([]int64, 0)
			resolveTimes := make([]int64, 0)

			contacts, err := businessRef.Collection(db.Directory).Where("associateIDs", "array-contains", associate.Id).Documents(ctx).GetAll()

			if err != nil {
				continue
			}

			for _, contact := range contacts {
				cases := findCasesForContact(contact.Ref.ID, acceptedCasesToDate)
				for _, caseItem := range cases {
					openedDate := caseItem.OpenedDate
					acceptDate := caseItem.AcceptedDate
					closedDate := caseItem.ClosedDate
					if openedDate != nil && acceptDate != nil {
						duration := acceptDate.Sub(*openedDate) / time.Millisecond
						acceptTimes = append(acceptTimes, int64(duration))

						respondedMessages, err := messagesRef.Where("textSessionId", "==", caseItem.TextSessionId).
							Where("sender.uid", "==", associate.Id).
							Where("createdDate", ">=", acceptDate).
							OrderBy("createdDate", firestore.Asc).
							Select("createdDate").
							Limit(1).Documents(ctx).GetAll()

						if err == nil && len(respondedMessages) > 0 {
							createdDate := respondedMessages[0].Data()["createdDate"].(time.Time)
							duration := createdDate.Sub(*openedDate) / time.Millisecond
							responseTimes = append(responseTimes, int64(duration))
						}
					}
					if caseItem.ClosedBy.Id == associate.Id {
						userPerf.ClosedCases += 1
						if openedDate != nil && closedDate != nil {
							duration := int64(closedDate.Sub(*openedDate) / time.Millisecond)
							resolveTimes = append(resolveTimes, duration)
						}
					}
				}
				userPerf.AcceptTime = CalculateAverage(acceptTimes)
				userPerf.ResponseTime = CalculateAverage(responseTimes)
				userPerf.ResolveTime = CalculateAverage(resolveTimes)
			}

			usersPerfs = append(usersPerfs, &userPerf)
		}

		if stats.Weekday > 0 {
			statsSnapshot, err = staffPerformanceRef.Doc(req.BeforeYesterdayID).Get(ctx)
			if err == nil && statsSnapshot.Exists() {
				documentIterator := statsSnapshot.Ref.Collection(db.Associates).Documents(ctx)
				for {
					doc, err := documentIterator.Next()
					if err == iterator.Done {
						break
					}
					if err != nil {
						continue
					}
					var prevUserPerf *staffPerfItem
					err = doc.DataTo(&prevUserPerf)
					if err != nil {
						continue
					}
					prevUserPerf.AssociateId = doc.Ref.ID
					if userPerf := findUserPerformance(usersPerfs, prevUserPerf.AssociateId); userPerf != nil {
						userPerf.Activity += prevUserPerf.Activity
						userPerf.ClosedCases += prevUserPerf.ClosedCases
						userPerf.Rating = (userPerf.Rating + prevUserPerf.Rating) / 2
						userPerf.AcceptTime = (userPerf.AcceptTime + prevUserPerf.AcceptTime) / 2
						userPerf.ResponseTime = (userPerf.ResponseTime + prevUserPerf.ResponseTime) / 2
						userPerf.ResolveTime = (userPerf.ResolveTime + prevUserPerf.ResolveTime) / 2
					}
				}
			}
		}

		associatesRef := yesterdayStatsRef.Collection(db.Associates)
		batch := client.Batch().Set(yesterdayStatsRef, stats)
		for _, userPerf := range usersPerfs {
			batch.Set(associatesRef.Doc(userPerf.AssociateId), userPerf)
		}
		_, err = batch.Commit(ctx)
		if err != nil {
			msq := fmt.Sprintf("failed to set stats for business:%s, date:%v, err:%v\n", req.BusinessID, req.YesterdayDate, err)
			return errors2.Wrap(err, msq)
		}
		return nil
	}
	return IterateSettings(ctx, client, execFunc)
}

func findUserPerformance(perfs []*staffPerfItem, associateId string) *staffPerfItem {
	for _, perf := range perfs {
		if perf.AssociateId == associateId {
			return perf
		}
	}
	return nil
}

func findCasesForContact(contactId string, cases []*firestore.DocumentSnapshot) []*model.Case {
	contactCases := make([]*model.Case, 0)
	for _, doc := range cases {
		var caseData *model.Case
		err := doc.DataTo(&caseData)
		if err != nil {
			continue
		}
		if contactId == caseData.Associate.Id {
			contactCases = append(contactCases, caseData)
		}
	}
	return contactCases
}
