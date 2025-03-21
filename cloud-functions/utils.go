package cloud_functions

import (
	"strings"
	"time"

	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
)

func NowWithOffset(tzOffset string) *time.Time {
	if strings.HasPrefix(tzOffset, "UTC") {
		tzOffset = strings.TrimPrefix(tzOffset, "UTC")
	}
	now := time.Now()
	tz, err := time.Parse("-07:00", tzOffset)
	if err != nil {
		return &now
	}
	inLoc := now.In(tz.Location())
	return &inLoc
}

func Min(array []int64) (int, int64) {
	minIndex := 0
	min := array[minIndex]
	for index, value := range array {
		if min > value {
			minIndex = index
			min = value
		}
	}
	return minIndex, min
}

func Max(array []int64) (int, int64) {
	maxIndex := 0
	max := array[maxIndex]
	for index, value := range array {
		if max < value {
			maxIndex = index
			max = value
		}
	}
	return maxIndex, max
}

func RemoveElementAt(items []int64, index int) {
	items[index] = items[len(items)-1]
	items[len(items)-1] = 0
	items = items[:len(items)-1]
}

func CalculateAverage(items []int64) int64 {
	if len(items) == 0 {
		return 0
	}
	sum := int64(0)
	if len(items) > 2 {
		minIndex, _ := Min(items)
		maxIndex, _ := Max(items)
		RemoveElementAt(items, minIndex)
		RemoveElementAt(items, maxIndex)
	}
	for _, item := range items {
		sum += item
	}
	return sum / int64(len(items))
}

func CalculateStuffActivity(wslog []*model.WorkingStatus, sunday time.Time, today *time.Time) (activity int64) {
	wsactive := make([]*time.Time, 0)
	wsabsent := make([]*time.Time, 0)
	for indx, ws := range wslog {
		if indx == 0 && ws.Type == 2 {
			// eliminate first log entry corresponding to absent status
			continue
		}
		if ws.Type == 1 {
			wsactive = append(wsactive, ws.Time)
			if indx == len(wslog)-1 {
				wsabsent = append(wsabsent, today)
			}
		} else {
			wsabsent = append(wsabsent, ws.Time)
		}
	}
	wsactiveLength := len(wsactive)
	for indx, absent := range wsabsent {
		if indx == wsactiveLength {
			break
		}
		activeTime := wsactive[indx]
		if activeTime.Before(sunday) { // cut-off to only include past 7 days
			activeTime = &sunday
		}
		activity += int64(absent.Sub(*activeTime) / time.Millisecond)
	}
	return
}
