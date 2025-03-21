package cloud_functions

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
)

// FirestoreChannelMessageEvent is the payload of a Firestore event.
type FirestoreChannelMessageEvent struct {
	OldValue   FirestoreChannelMessageValue `json:"oldValue"`
	Value      FirestoreChannelMessageValue `json:"value"`
	UpdateMask struct {
		FieldPaths []string `json:"fieldPaths"`
	} `json:"updateMask"`
}

// FirestoreChannelMessageValue holds Firestore fields.
type FirestoreChannelMessageValue struct {
	CreateTime time.Time `json:"createTime"`
	// Fields is the data for this value. The type depends on the format of your
	// database. Log an interface{} value and inspect the result to see a JSON
	// representation of your database fields.
	Fields     channelMessage `json:"fields"`
	Name       string         `json:"name"`
	UpdateTime time.Time      `json:"updateTime"`
}

type channelMessage struct {
	ID   string `json:"id"`
	Text struct {
		Value string `json:"stringValue"`
	} `json:"text"`
}

type lastChannelMessage struct {
	ID   string `firestore:"id"`
	Text string `firestore:"text"`
}

func businessesChannelsMessagesOnCreate(ctx context.Context, client *firestore.Client,
	channelRef *firestore.DocumentRef, message channelMessage) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	computeUnreadUIDs := func(memberIDs []string, presenceUIDs []string) []string {
		var unreadUIDs []string
		for _, uid := range memberIDs {
			for _, pUID := range presenceUIDs {
				if uid == pUID {
					continue
				}
			}
			unreadUIDs = append(unreadUIDs, uid)
		}
		return unreadUIDs
	}

	err := client.RunTransaction(ctx, func(ctx context.Context, transaction *firestore.Transaction) error {
		snapshot, err := transaction.Get(channelRef)
		if err != nil {
			return err
		}
		var channel *model.Channel
		err = snapshot.DataTo(&channel)
		if err != nil {
			return err
		}
		presentUIDs := channel.Presence.UIDs()
		unreadUIDs := computeUnreadUIDs(channel.MemberIDs, presentUIDs)
		if channel.Unread == nil {
			channel.Unread = new(model.Unread)
		}
		var updates []firestore.Update
		for _, uid := range unreadUIDs {
			update := firestore.Update{
				FieldPath: firestore.FieldPath{"unread", uid},
				Value:     firestore.Increment(1),
			}
			updates = append(updates, update)
		}
		elems := []firestore.Update{
			{
				Path: "lastMessage",
				Value: &lastChannelMessage{
					ID:   message.ID,
					Text: message.Text.Value,
				},
			},
			{
				Path:  "updatedDate",
				Value: time.Now(),
			},
		}
		updates = append(updates, elems...)
		err = transaction.Update(channelRef, updates)
		if err != nil {
			return err
		}
		return nil
	}, firestore.MaxAttempts(1))
	return err
}

func businessesChannelsMessagesOnDelete(ctx context.Context, client *firestore.Client, event FirestoreChannelMessageEvent) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	unreadUIDs := func(unread *model.Unread) []string {
		var uids []string
		if unread == nil {
			return uids
		}
		for uid := range *unread {
			uids = append(uids, uid)
		}
		return uids
	}

	fullPath := strings.Split(event.OldValue.Name, "/documents/")[1]
	pathParts := strings.Split(fullPath, "/")
	messageId := pathParts[len(pathParts)-1]
	channelDocPath := strings.Join(pathParts[:(len(pathParts)-2)], "/")
	err := client.RunTransaction(ctx, func(ctx context.Context, transaction *firestore.Transaction) error {
		snapshot, err := transaction.Get(client.Doc(channelDocPath))
		if err != nil {
			return err
		}
		var channel *model.Channel
		err = snapshot.DataTo(&channel)
		if err != nil {
			return err
		}
		var updates []firestore.Update
		for _, uid := range unreadUIDs(channel.Unread) {
			update := firestore.Update{
				FieldPath: firestore.FieldPath{"unread", uid},
				Value:     firestore.Increment(-1),
			}
			updates = append(updates, update)
		}

		if channel.LastMessage != nil {
			query := client.Doc(channelDocPath).Collection("channelMessages").OrderBy("createdDate", firestore.Desc).Limit(1)
			documents, err := transaction.Documents(query).GetAll()
			if err != nil {
				return err
			}
			if len(documents) == 0 {
				updates = append(updates, firestore.Update{
					Path:  "lastMessage",
					Value: nil,
				})
			}
			if channel.LastMessage.ID == messageId {
				for _, doc := range documents {
					var message *model.ChannelMessage
					if err := doc.DataTo(&message); err == nil {
						message.ID = doc.Ref.ID
						elem := firestore.Update{
							Path: "lastMessage",
							Value: &lastChannelMessage{
								ID:   message.ID,
								Text: message.Text,
							},
						}
						updates = append(updates, elem)
					}
				}
			}
		}
		err = transaction.Update(client.Doc(channelDocPath), updates)
		if err != nil {
			return err
		}
		return nil
	}, firestore.MaxAttempts(1))
	return err
}
