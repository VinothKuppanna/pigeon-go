package cloud_functions

import (
	"context"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

// settingsEvent is the payload of a Firestore event.
type settingsEvent struct {
	OldValue   settingsValue `json:"oldValue"`
	Value      settingsValue `json:"value"`
	UpdateMask struct {
		FieldPaths []string `json:"fieldPaths"`
	} `json:"updateMask"`
}

func (e *settingsEvent) accessProtectionActive() bool {
	return e.Value.Fields.AccessProtection.MapValue.Fields.Active.BooleanValue
}

func (e *settingsEvent) accessProtectionCode() string {
	return e.Value.Fields.AccessProtection.MapValue.Fields.Code.StringValue
}

// settingsValue holds Firestore fields.
type settingsValue struct {
	CreateTime time.Time `json:"createTime"`
	// Fields is the data for this value. The type depends on the format of your
	// database. Log an interface{} value and inspect the result to see a JSON
	// representation of your database fields.
	Fields     settingsData `json:"fields"`
	Name       string       `json:"name"`
	UpdateTime time.Time    `json:"updateTime"`
}

// settingsData represents a value from Firestore.
type settingsData struct {
	AccessProtection struct {
		MapValue struct {
			Fields struct {
				Active struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"active"`
				Code struct {
					StringValue string `json:"stringValue"`
				} `json:"code"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"accessProtection"`
	AlwaysOpen struct {
		BooleanValue bool `json:"booleanValue"`
	} `json:"alwaysOpen"`
	Appoints struct {
		MapValue struct {
			Fields struct {
				Active struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"active"`
				AppointDays struct {
					ArrayValue struct {
						Values []struct {
							MapValue struct {
								Fields struct {
									Active struct {
										BooleanValue bool `json:"booleanValue"`
									} `json:"active"`
									End struct {
										StringValue string `json:"stringValue"`
									} `json:"end"`
									From struct {
										StringValue string `json:"stringValue"`
									} `json:"from"`
									Name struct {
										StringValue string `json:"stringValue"`
									} `json:"name"`
									Start struct {
										StringValue string `json:"stringValue"`
									} `json:"start"`
									To struct {
										StringValue string `json:"stringValue"`
									} `json:"to"`
								} `json:"fields"`
							} `json:"mapValue"`
						} `json:"values"`
					} `json:"arrayValue"`
				} `json:"appointDays"`
				Contact struct {
					NullValue interface{} `json:"nullValue"`
				} `json:"contact"`
				Period struct {
					IntegerValue string `json:"integerValue"`
				} `json:"period"`
				Remind struct {
					NullValue interface{} `json:"nullValue"`
				} `json:"remind"`
				SyncGoogle struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"syncGoogle"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"appoints"`
	AutoAlert struct {
		MapValue struct {
			Fields struct {
				Active struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"active"`
				Contacts struct {
					ArrayValue struct {
					} `json:"arrayValue"`
				} `json:"contacts"`
				Emails struct {
					ArrayValue struct {
					} `json:"arrayValue"`
				} `json:"emails"`
				IdleTime struct {
					IntegerValue string `json:"integerValue"`
				} `json:"idleTime"`
				Repeat struct {
					IntegerValue string `json:"integerValue"`
				} `json:"repeat"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"autoAlert"`
	Away struct {
		MapValue struct {
			Fields struct {
				Active struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"active"`
				AwayMessage struct {
					NullValue interface{} `json:"nullValue"`
				} `json:"awayMessage"`
				ClosedMessage struct {
					NullValue interface{} `json:"nullValue"`
				} `json:"closedMessage"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"away"`
	Backup struct {
		MapValue struct {
			Fields struct {
				Active struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"active"`
				AutoForward struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"autoForward"`
				AutoGroup struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"autoGroup"`
				Contact struct {
					NullValue interface{} `json:"nullValue"`
				} `json:"contact"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"backup"`
	Contacts struct {
		MapValue struct {
			Fields struct {
				AutoSharing struct {
					BooleanValue bool `json:"booleanValue"`
				} `json:"autoSharing"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"contacts"`
	TimeZone struct {
		StringValue string `json:"stringValue"`
	} `json:"timeZone"`
	TimeZoneIndex struct {
		IntegerValue string `json:"integerValue"`
	} `json:"timeZoneIndex"`
	WorkingDays struct {
		ArrayValue struct {
			Values []struct {
				MapValue struct {
					Fields struct {
						Active struct {
							BooleanValue bool `json:"booleanValue"`
						} `json:"active"`
						End struct {
							StringValue string `json:"stringValue"`
						} `json:"end"`
						From struct {
							StringValue string `json:"stringValue"`
						} `json:"from"`
						Name struct {
							StringValue string `json:"stringValue"`
						} `json:"name"`
						Start struct {
							StringValue string `json:"stringValue"`
						} `json:"start"`
						To struct {
							StringValue string `json:"stringValue"`
						} `json:"to"`
					} `json:"fields"`
				} `json:"mapValue"`
			} `json:"values"`
		} `json:"arrayValue"`
	} `json:"workingDays"`
}

func businessSettingsWrite(ctx context.Context, db *db.Firestore, event *settingsEvent) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fullPath := strings.Split(event.Value.Name, "/documents/")[1]
	children := strings.Split(fullPath, "/")
	businessID := strings.Join(children[1:], "/")
	if event.accessProtectionChanged() {
		log.Println("access protection changed")
		if event.accessProtectionCodeChanged() {
			updates := []firestore.Update{{
				Path:  "accessProtected",
				Value: true,
			}}
			count := 0
			batch := db.Batch()
			refIterator := db.BusinessCustomers(businessID).DocumentRefs(ctx)
			for {
				ref, err := refIterator.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					return err
				}
				customerID := ref.ID
				batch.Update(db.CustomerBusiness(customerID, businessID), updates)
				batch.Delete(db.CustomerBusinessAccess(customerID, businessID))
				count++
				if count == 500 {
					_, _ = batch.Commit(ctx)
					count = 0
					batch = db.Batch()
				}
			}
			if count > 0 {
				_, _ = batch.Commit(ctx)
			}
		}
		if event.accessProtectionActiveChanged() {
			data := firestore.Update{
				Path:  "accessProtected",
				Value: true,
			}
			if !event.accessProtectionActive() {
				log.Println("access protection disabled")
				data.Value = firestore.Delete
			}
			updates := []firestore.Update{data}
			_, err := db.Business(businessID).Update(ctx, updates)
			if err != nil {
				return errors.Wrap(err, "failed to update business")
			}
		}
	}
	return nil
}

func (e *settingsEvent) accessProtectionActiveChanged() bool {
	oldProtectionEnabled := e.OldValue.Fields.AccessProtection.MapValue.Fields.Active.BooleanValue
	newProtectionEnabled := e.Value.Fields.AccessProtection.MapValue.Fields.Active.BooleanValue
	return oldProtectionEnabled != newProtectionEnabled
}

func (e *settingsEvent) accessProtectionCodeChanged() bool {
	oldAccessCode := e.OldValue.Fields.AccessProtection.MapValue.Fields.Code.StringValue
	newAccessCode := e.Value.Fields.AccessProtection.MapValue.Fields.Code.StringValue
	return oldAccessCode != newAccessCode
}

func (e *settingsEvent) accessProtectionChanged() bool {
	return e.accessProtectionActiveChanged() || e.accessProtectionCodeChanged()
}
