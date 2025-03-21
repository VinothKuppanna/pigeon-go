package cloud_functions

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/messaging"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
)

func notifyNewChannelMessage(ctx context.Context, err error, channelRef *firestore.DocumentRef, message channelMessage) error {
	snapshot, err := channelRef.Get(ctx)
	if err != nil {
		return err
	}
	var channel *model.Channel
	err = snapshot.DataTo(&channel)
	if err != nil {
		return err
	}
	channel.ID = channelRef.ID
	if unread := channel.Unread; unread != nil {
		data := map[string]string{
			"businessId": channel.Business.Id,
			"channelId":  channel.ID,
			"messageId":  message.ID,
			"title":      channel.Name,
			"message":    message.Text.Value,
			"photoUrl":   channel.ImageUrl,
			"category":   "CHANNEL_MESSAGE_CATEGORY",
		}
		notification := messaging.Notification{
			Title:    channel.Name,
			Body:     message.Text.Value,
			ImageURL: channel.ImageUrl,
		}
		options := messaging.FCMOptions{
			AnalyticsLabel: "CHANNEL_MESSAGE_CATEGORY",
		}
		androidConfig := messaging.AndroidConfig{
			Priority: "normal",
		}
		apnsConfig := messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:    "chats.caf",
					Category: "CHANNEL_MESSAGE_CATEGORY",
				},
			},
		}
		var tokens []string
		var messages []*messaging.Message
		for uid, count := range *unread {
			// check channel is muted
			if channel.MutedBy.Contains(uid) {
				continue
			}

			// check user is muted globally
			userSnapshot, err := client.Collection("users").Doc(uid).Get(ctx)
			if err != nil {
				log.Println("user data error.", err)
				continue
			}
			var user *model.User
			err = userSnapshot.DataTo(&user)
			if err != nil {
				log.Println("user data error.", err)
				continue
			}
			user.Id = uid
			if user.IsMuted() {
				continue
			}
			tokensSnapshot, err := client.Collection("fcmTokens").Where("uid", "==", uid).Documents(ctx).GetAll()
			if err != nil {
				log.Println("failed to get FCM tokens for UID:", uid, err)
				continue
			}
			for _, snapshot := range tokensSnapshot {
				var token *model.FCMToken
				err := snapshot.DataTo(&token)
				if err != nil {
					log.Println("failed to get FCM token", err)
					continue
				}
				token.ID = snapshot.Ref.ID
				message := messaging.Message{
					Data:       data,
					FCMOptions: &options,
					Token:      token.ID,
				}
				switch token.Platform {
				case "web":
					//message.Webpush = &messaging.WebpushConfig{
					//	Data:         nil,
					//	Notification: nil,
					//	FCMOptions:   nil,
					//}
					continue
				case "android":
					message.Android = &androidConfig
				case "ios":
					message.Notification = &notification
					badge := int(count)
					apnsConfig.Payload.Aps.Badge = &badge
					message.APNS = &apnsConfig
				default:
					message.Android = &androidConfig
				}
				messages = append(messages, &message)
				tokens = append(tokens, token.ID)
			}
		}
		msgService, err := app.Messaging(ctx)
		if err != nil {
			return err
		}
		chunked := chunkMessages(messages)
		for index, msgs := range chunked {
			response, err := msgService.SendAll(ctx, msgs)
			if err != nil {
				return err
			}
			log.Println("SendAll: ", response.SuccessCount, "/", response.FailureCount)
			batch := client.Batch()
			offset := index * len(response.Responses)
			for respIndx, response := range response.Responses {
				if !response.Success {
					log.Println("Unsuccessful FCM response:", err)
					if messaging.IsUnregistered(err) {
						log.Println("Token is invalid")
						tokenID := tokens[respIndx+offset]
						batch.Delete(client.Collection("fcmTokens").Doc(tokenID))
					}
				}
			}
			result, err := batch.Commit(ctx)
			if err != nil {
				log.Println("Clear invalid FCM tokens.", err)
			}
			log.Println("Cleared FCM tokens:", len(result))
		}
	}
	return err
}

func chunkMessages(messages []*messaging.Message) (divided [][]*messaging.Message) {
	chunkSize := 500
	for i := 0; i < len(messages); i += chunkSize {
		end := i + chunkSize
		if end > len(messages) {
			end = len(messages)
		}
		divided = append(divided, messages[i:end])
	}
	return
}
