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

// businessEvent is the payload of a Firestore event.
type businessEvent struct {
	OldValue   businessValue `json:"oldValue"`
	Value      businessValue `json:"value"`
	UpdateMask struct {
		FieldPaths []string `json:"fieldPaths"`
	} `json:"updateMask"`
}

// businessValue holds Firestore fields.
type businessValue struct {
	CreateTime time.Time `json:"createTime"`
	// Fields is the data for this value. The type depends on the format of your
	// database. Log an interface{} value and inspect the result to see a JSON
	// representation of your database fields.
	Fields     businessData `json:"fields"`
	Name       string       `json:"name"`
	UpdateTime time.Time    `json:"updateTime"`
}

type businessData struct {
	AccessProtected struct {
		BooleanValue bool `json:"booleanValue"`
	} `json:"accessProtected"`
	Address struct {
		StringValue string `json:"stringValue"`
	} `json:"address"`
	BusinessCategory struct {
		MapValue struct {
			Fields struct {
				Id struct {
					StringValue string `json:"stringValue"`
				} `json:"id"`
				Index struct {
					IntegerValue string `json:"integerValue"`
				} `json:"index"`
				Name struct {
					StringValue string `json:"stringValue"`
				} `json:"name"`
				Tag struct {
					StringValue string `json:"stringValue"`
				} `json:"tag"`
				Tags struct {
					ArrayValue struct {
						Values []struct {
							StringValue string `json:"stringValue"`
						} `json:"values"`
					} `json:"arrayValue"`
				} `json:"tags"`
			} `json:"fields"`
		} `json:"mapValue"`
	} `json:"businessCategory"`
	City struct {
		StringValue string `json:"stringValue"`
	} `json:"city"`
	Country struct {
		NullValue interface{} `json:"nullValue"`
	} `json:"country"`
	CreatedDate struct {
		TimestampValue time.Time `json:"timestampValue"`
	} `json:"createdDate"`
	Email struct {
		StringValue string `json:"stringValue"`
	} `json:"email"`
	Employees struct {
		StringValue string `json:"stringValue"`
	} `json:"employees"`
	Name struct {
		StringValue string `json:"stringValue"`
	} `json:"name"`
	Opened struct {
		BooleanValue bool `json:"booleanValue"`
	} `json:"opened"`
	Phone struct {
		StringValue string `json:"stringValue"`
	} `json:"phone"`
	State struct {
		StringValue string `json:"stringValue"`
	} `json:"state"`
	Street struct {
		StringValue string `json:"stringValue"`
	} `json:"street"`
	TaxId struct {
		StringValue string `json:"stringValue"`
	} `json:"taxId"`
	Verified struct {
		BooleanValue bool `json:"booleanValue"`
	} `json:"verified"`
	Website struct {
		StringValue string `json:"stringValue"`
	} `json:"website"`
	ZipCode struct {
		StringValue string `json:"stringValue"`
	} `json:"zipCode"`
}

func (be *businessEvent) Business() *businessData {
	return &be.Value.Fields
}

func (be *businessEvent) String() string {
	return be.Business().String()
}

func (b *businessData) String() string {
	return b.Name.StringValue
}

// todo: if business name has been changed
// 1. update chats
// 2. update business contacts
// 3. update appointments
// 4. update customer's business/contacts
func businessUpdated(ctx context.Context, db *db.Firestore, event *businessEvent) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fullPath := strings.Split(event.Value.Name, "/documents/")[1]
	children := strings.Split(fullPath, "/")
	businessID := strings.Join(children[1:], "/")
	log.Println("business changed ", businessID)

	var accessUpdate *firestore.Update
	var nameUpdate *firestore.Update

	if event.accessProtectedChanged() {
		value := event.Business().AccessProtected.BooleanValue
		accessUpdate = &firestore.Update{
			Path:  "accessProtected",
			Value: value}
		if !value {
			accessUpdate.Value = firestore.Delete
		}
	}

	if event.nameChanged() {
		nameUpdate = &firestore.Update{
			Path:  "name",
			Value: event.Business().Name.StringValue,
		}
	}

	if accessUpdate == nil && nameUpdate == nil {
		return nil
	}

	count := 0
	batch := db.Batch()
	documentRefs := db.BusinessCustomers(businessID).DocumentRefs(ctx)
	for {
		next, err := documentRefs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		var updates []firestore.Update
		customerID := next.ID
		if accessUpdate != nil {
			if _, err = db.CustomerBusinessAccess(customerID, businessID).Get(ctx); err != nil {
				updates = append(updates, *accessUpdate)
			}
		}
		if nameUpdate != nil {
			updates = append(updates, *nameUpdate)
		}
		if len(updates) > 0 {
			batch.Update(db.CustomerBusiness(customerID, businessID), updates)
			count++
			if count == 500 {
				_, err = batch.Commit(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to update customers' business")
				}
				count = 0
				batch = db.Batch()
			}
		}
	}
	if count > 0 {
		_, err := batch.Commit(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to update customers' business")
		}
	}
	return nil
}

func (be *businessEvent) accessProtectedChanged() bool {
	oldAccessProtected := be.OldValue.Fields.AccessProtected.BooleanValue
	newAccessProtected := be.Value.Fields.AccessProtected.BooleanValue
	return oldAccessProtected != newAccessProtected
}

func (be *businessEvent) nameChanged() bool {
	oldName := be.OldValue.Fields.Name.StringValue
	newName := be.Value.Fields.Name.StringValue
	return oldName != newName
}
