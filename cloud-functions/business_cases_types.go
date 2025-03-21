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
)

type casesTypesStats struct {
	Cases       map[string]int64 `firestore:"cases"`
	CreatedDate time.Time        `firestore:"createdDate"`
}

func businessCasesTypes(ctx context.Context, client *firestore.Client) error {
	execFunc := func(req *SettingsRequest) error {
		businessRef := client.Collection(db.Businesses).Doc(req.BusinessID)
		casesTypesStatsRef := businessRef.Collection(db.CasesTypesStats)
		statsSnapshot, err := casesTypesStatsRef.Doc(req.YesterdayID).Get(ctx)
		if statsSnapshot != nil && statsSnapshot.Exists() {
			return errors.New(fmt.Sprintf("stats exist for this time ID:%s", req.YesterdayID))
		}
		stats := casesTypesStats{
			Cases:       make(map[string]int64),
			CreatedDate: *req.Yesterday,
		}
		casesRef := businessRef.Collection(db.Cases)
		all, err := casesRef.Where("openedDate", ">=", req.YesterdayDate).
			Where("openedDate", "<", req.TodayDate).
			Select("associate").Documents(ctx).GetAll()
		if err != nil {
			log.Println("opened cases continue with error", err)
			return errors.New("")
		}
		directoryRef := businessRef.Collection(db.Directory)
		groupedCases := groupByAssociateId(all)
		for id, value := range groupedCases {
			documentSnapshot, err := directoryRef.Doc(id).Get(ctx)
			if err != nil {
				if other, ok := stats.Cases["Other"]; ok {
					stats.Cases["Other"] = other + value
				} else {
					stats.Cases["Other"] = value
				}
				log.Println("get contact with error", err)
				continue
			}
			docData := documentSnapshot.Data()
			if path, ok := docData["path"].([]string); ok && len(path) > 0 {
				department := path[len(path)-1]
				if depValue, ok := stats.Cases[department]; ok {
					stats.Cases[department] = depValue + value
				} else {
					stats.Cases[department] = value
				}
			} else if other, ok := stats.Cases["Other"]; ok {
				stats.Cases["Other"] = other + value
			} else {
				stats.Cases["Other"] = value
			}
		}
		statsSnapshot, err = casesTypesStatsRef.Doc(req.BeforeYesterdayID).Get(ctx)
		if err == nil && statsSnapshot.Exists() {
			var prevStats *casesTypesStats
			if err = statsSnapshot.DataTo(&prevStats); err == nil {
				for ct, prevCount := range prevStats.Cases {
					if count, ok := stats.Cases[ct]; ok {
						stats.Cases[ct] = count + prevCount
					} else {
						stats.Cases[ct] = prevCount
					}
				}
			}
		}
		_, err = casesTypesStatsRef.Doc(req.YesterdayID).Set(ctx, stats)
		if err != nil {
			msg := fmt.Sprintf("failed to set stats for business:%s, date:%v, err:%v\n", req.BusinessID, req.YesterdayDate, err)
			return errors2.Wrap(err, msg)
		}
		return nil
	}
	return IterateSettings(ctx, client, execFunc)
}

func groupByAssociateId(all []*firestore.DocumentSnapshot) map[string]int64 {
	group := make(map[string]int64)
	for _, item := range all {
		if associate, ok := item.Data()["associate"].(map[string]interface{}); ok {
			if id, ok := associate["id"].(string); ok {
				if _, ok := group[id]; ok {
					group[id] += 1
				} else {
					group[id] = 1
				}
			}
		}
	}
	return group
}
