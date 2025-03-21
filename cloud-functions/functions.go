package cloud_functions

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/storage"

	"log"
	"os"
	"strings"

	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

var (
	projectID          = os.Getenv("PROJECT_ID")
	projectNumber      = os.Getenv("PROJECT_NUMBER")
	serviceAccountName = fmt.Sprintf("%s-compute@developer.gserviceaccount.com", projectNumber)
	serviceAccountID   = fmt.Sprintf(
		"projects/%s/serviceAccounts/%s",
		projectID,
		serviceAccountName,
	)
)

var (
	app *firebase.App
	// client is a Firestore client, reused between function invocations.
	client *firestore.Client
	// client is a Firestore client, reused between function invocations.
	//rtdb *db.Client
	// storage storageClient is Storage client, reused between function invocations
	storageClient *storage.Client
	authClient    *auth.Client
	// iamService is a client for calling the signBlob API.
	iamService *iam.Service
	//defaultCredentials *google.Credentials
)

func init() {
	// Use the application default credentials.
	conf := &firebase.Config{ProjectID: projectID, ServiceAccountID: serviceAccountID}

	// Use context.Background() because the app/client should persist across
	// invocations.
	ctx := context.Background()

	//defaultCredentials, err := google.FindDefaultCredentials(ctx)
	//if err != nil {
	//	log.Fatalf("google.FindDefaultCredentials: %v", err)
	//}

	service, err := iam.NewService(ctx, option.WithScopes(iam.CloudPlatformScope))
	if err != nil {
		log.Fatalf("iam.NewService: %v", err)
	}
	iamService = service

	app, err = firebase.NewApp(ctx, conf)
	if err != nil {
		log.Fatalf("firebase.NewApp: %v", err)
	}

	authClient, err = app.Auth(ctx)
	if err != nil {
		log.Fatalf("app.Auth: %v", err)
	}

	client, err = app.Firestore(ctx)
	if err != nil {
		log.Fatalf("app.Firestore: %v", err)
	}

	storageClient, err = app.Storage(ctx)
	if err != nil {
		log.Fatalf("app.Storage: %v", err)
	}
}

func BusinessOnUpdate(ctx context.Context, event businessEvent) error {
	return businessUpdated(ctx, &db.Firestore{Client: client}, &event)
}

func BusinessSettingsWrite(ctx context.Context, event settingsEvent) error {
	return businessSettingsWrite(ctx, &db.Firestore{Client: client}, &event)
}

func StatsCalculateRating(ctx context.Context, msg interface{}) error {
	log.Println("calculating rating data...")
	return calculateRating(ctx, client)
}

func StatsBusinessCases(ctx context.Context, msg interface{}) error {
	log.Println("calculating cases data...")
	return businessCases(ctx, client)
}

func StatsInteractions(ctx context.Context, msg interface{}) error {
	log.Println("calculating interactions data...")
	return interactions(ctx, client)
}

func StatsBusinessCasesTypes(ctx context.Context, msg interface{}) error {
	log.Println("calculating cases types data...")
	return businessCasesTypes(ctx, client)
}

func StatsResponseTime(ctx context.Context, msg interface{}) error {
	log.Println("calculating response time data...")
	return responseTime(ctx, client)
}

func StatsStaffPerformance(ctx context.Context, msg interface{}) error {
	log.Println("calculating staff performance data...")
	return staffPerformance(ctx, client)
}

// ImageFileUploaded Cloud Storage
func ImageFileUploaded(ctx context.Context, event model.GCSEvent) error {
	log.Println("processing uploaded image")
	return imageFileUploaded(ctx, iamService, authClient, client, storageClient, event)
}

func BusinessesChannelsMessagesOnCreate(ctx context.Context, event FirestoreChannelMessageEvent) error {
	fullPath := strings.Split(event.Value.Name, "/documents/")[1]
	pathParts := strings.Split(fullPath, "/")
	messageId := pathParts[len(pathParts)-1]
	channelDoc := strings.Join(pathParts[:(len(pathParts)-2)], "/")
	channelRef := client.Doc(channelDoc)
	message := event.Value.Fields
	message.ID = messageId
	err := businessesChannelsMessagesOnCreate(ctx, client, channelRef, message)
	if err != nil {
		return err
	}
	return notifyNewChannelMessage(ctx, err, channelRef, message)
}

func BusinessesChannelsMessagesOnDelete(ctx context.Context, event FirestoreChannelMessageEvent) error {
	return businessesChannelsMessagesOnDelete(ctx, client, event)
}

func BusinessesChannelsOnDelete(ctx context.Context, event FirestoreChannelEvent) error {
	return businessesChannelsOnDelete(ctx, client, event)
}
