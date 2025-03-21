package cloud_functions

import (
	"testing"
	"time"

	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
)

func TestCalculateStuffActivity(t *testing.T) {
	type args struct {
		wslog  []*model.WorkingStatus
		sunday time.Time
		today  *time.Time
	}
	now := time.Date(2021, 1, 6, 0, 0, 0, 0, time.Local)
	sunday := now.AddDate(0, 0, -int(now.Weekday()))
	active1 := sunday.Add(time.Hour * 8)
	absent1 := active1.Add(time.Hour * 2)
	active2 := absent1.Add(time.Hour * 3)
	absent2 := active2.Add(time.Hour * 5)
	active3 := absent2.Add(time.Hour * 5)
	tests := []struct {
		name         string
		args         args
		wantActivity int64
	}{
		{
			name: "first active",
			args: args{
				wslog: []*model.WorkingStatus{
					{Time: &sunday, Type: 2},
					{Time: &active1, Type: 1},
					{Time: &absent1, Type: 2},
					{Time: &active2, Type: 1},
					{Time: &absent2, Type: 2},
					{Time: &active3, Type: 1},
				},
				sunday: sunday,
				today:  &now,
			},
			wantActivity: 201600000, // 56 hrs
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotActivity := CalculateStuffActivity(tt.args.wslog, tt.args.sunday, tt.args.today); gotActivity != tt.wantActivity {
				t.Errorf("CalculateStuffActivity() = %v, want %v", gotActivity, tt.wantActivity)
			}
		})
	}
}
