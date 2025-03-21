package cloud_functions

import (
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/iterator"
	"strings"
	"time"
)

// FirestoreChannelEvent is the payload of a Firestore event.
type FirestoreChannelEvent struct {
	OldValue   FirestoreChannelValue `json:"oldValue"`
	Value      FirestoreChannelValue `json:"value"`
	UpdateMask struct {
		FieldPaths []string `json:"fieldPaths"`
	} `json:"updateMask"`
}

// FirestoreChannelValue holds Firestore fields.
type FirestoreChannelValue struct {
	CreateTime time.Time `json:"createTime"`
	// Fields is the data for this value. The type depends on the format of your
	// database. Log an interface{} value and inspect the result to see a JSON
	// representation of your database fields.
	Fields     interface{} `json:"fields"`
	Name       string      `json:"name"`
	UpdateTime time.Time   `json:"updateTime"`
}

func businessesChannelsOnDelete(ctx context.Context, client *firestore.Client, event FirestoreChannelEvent) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fullPath := strings.Split(event.OldValue.Name, "/documents/")[1]
	channelRef := client.Doc(fullPath)
	refsIterator := channelRef.Collection("channelMessages").DocumentRefs(ctx)
	err := removeAllDocs(ctx, refsIterator)
	if err != nil {
		return err
	}
	refsIterator = channelRef.Collection("members").DocumentRefs(ctx)
	err = removeAllDocs(ctx, refsIterator)
	if err != nil {
		return err
	}
	return nil
}

func removeAllDocs(ctx context.Context, refsIterator *firestore.DocumentRefIterator) error {
	for {
		next, err := refsIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		_, _ = next.Delete(ctx, firestore.Exists)
	}
	return nil
}
