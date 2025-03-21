package cloud_functions

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	storage2 "cloud.google.com/go/storage"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/storage"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/db"
	"github.com/VinothKuppanna/pigeon-go/pkg/data/model"
	"google.golang.org/api/iam/v1"
)

const (
	substrChannels     = "/channels"
	substrImages       = "/images"
	substrProfilePhoto = "profilePhoto/"
	substrPhoto        = "/photo"
	prefixImage        = "image/"
	prefixThumb        = "thumb_"
	prefixNormal       = "normal_"
	cacheControl       = "public,max-age=3600"
	aclPublicRead      = "publicRead"
	thumbMaxWidth      = 200
	thumbMaxHeight     = 200
	normalMaxWidth     = 512
	normalMaxHeight    = 512
)

func imageFileUploaded(ctx context.Context, iamService *iam.Service, authClient *auth.Client,
	firestoreClient *firestore.Client, storageClient *storage.Client, event model.GCSEvent) error {
	filePath := event.Name
	profilePhoto := strings.Contains(filePath, substrProfilePhoto)
	contactPhoto := strings.Contains(filePath, substrPhoto)
	channelImage := isChannelImage(filePath)
	if !profilePhoto && !contactPhoto && !channelImage {
		log.Println("skip precessing. not profile photo")
		return nil
	}
	// image MIME type
	contentType := event.ContentType
	// exit if file is not an image
	if !strings.HasPrefix(contentType, prefixImage) {
		log.Println("skip precessing. not an image")
		return nil
	}
	// extract file dir and name
	fileDir := path.Dir(filePath)
	fileName := path.Base(filePath)
	// exit if image thumbs are being processed
	if strings.HasPrefix(fileName, prefixThumb) || strings.HasPrefix(fileName, prefixNormal) {
		log.Println("skip precessing. image thumbnails")
		return nil
	}

	if channelImage {
		return processChannelImage(ctx, firestoreClient, fileDir, fileName, filePath, event)
	}

	thumbFilePath := path.Clean(path.Join(fileDir, fmt.Sprintf("%s%s", prefixThumb, fileName)))
	normalFilePath := path.Clean(path.Join(fileDir, fmt.Sprintf("%s%s", prefixNormal, fileName)))
	// cloud storage
	bucket, err := storageClient.Bucket(event.Bucket)
	if err != nil {
		return fmt.Errorf("client.Bucket: %v", err)
	}
	ctx, cancelFunc := context.WithTimeout(ctx, time.Millisecond*10000)
	defer cancelFunc()

	srcFile := bucket.Object(filePath)
	thumbFile := bucket.Object(thumbFilePath)
	normalFile := bucket.Object(normalFilePath)

	// make thumbnail
	err = applyImageMagick(ctx, srcFile, thumbFile, "-", "-auto-orient", "-thumbnail", fmt.Sprintf("%dx%d>", thumbMaxWidth, thumbMaxHeight), "-")
	if err != nil {
		return fmt.Errorf("applyImageMagick: %v", err)
	}
	// make normal
	err = applyImageMagick(ctx, srcFile, normalFile, "-", "-auto-orient", "-thumbnail", fmt.Sprintf("%dx%d>", normalMaxWidth, normalMaxHeight), "-")
	if err != nil {
		return fmt.Errorf("applyImageMagick: %v", err)
	}

	err = srcFile.Delete(ctx)
	if err != nil {
		return fmt.Errorf("srcFile.Delete: %v", err)
	}

	attrsToUpdate := storage2.ObjectAttrsToUpdate{
		ContentType:   contentType,
		CacheControl:  cacheControl,
		PredefinedACL: aclPublicRead,
	}
	_, err = thumbFile.Update(ctx, attrsToUpdate)
	if err != nil {
		return fmt.Errorf("thumbFile.Update: %v", err)
	}
	_, err = normalFile.Update(ctx, attrsToUpdate)
	if err != nil {
		return fmt.Errorf("normalFile.Update: %v", err)
	}

	options := storage2.SignedURLOptions{
		GoogleAccessID: serviceAccountName,
		SignBytes: func(bytes []byte) ([]byte, error) {
			resp, err := iamService.Projects.ServiceAccounts.SignBlob(
				serviceAccountID,
				&iam.SignBlobRequest{BytesToSign: base64.StdEncoding.EncodeToString(bytes)},
			).Context(ctx).Do()
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(resp.Signature)
		},
		Method:  "GET",
		Expires: time.Now().AddDate(100, 0, 0),
	}

	thumbUrl, err := signedURLNormalized(thumbFile.BucketName(), thumbFile.ObjectName(), &options)
	if err != nil {
		return fmt.Errorf("signedURLNormalized: %v", err)
	}

	normalUrl, err := signedURLNormalized(normalFile.BucketName(), normalFile.ObjectName(), &options)
	if err != nil {
		return fmt.Errorf("signedURLNormalized: %v", err)
	}

	updates := []firestore.Update{
		{
			Path:  "photoUrl.thumbnail",
			Value: thumbUrl,
		},
		{
			Path:  "photoUrl.normal",
			Value: normalUrl,
		},
	}
	if profilePhoto {
		return updateUserProfile(ctx, authClient, firestoreClient, filePath, updates)
	}
	if contactPhoto {
		if err = updateContact(ctx, firestoreClient, fileDir, updates); err != nil {
			return err
		}
	}
	return nil
}

func processChannelImage(ctx context.Context, firestoreClient *firestore.Client,
	fileDir, fileName, filePath string, event model.GCSEvent) error {
	thumbFilePath := path.Clean(path.Join(fileDir, fmt.Sprintf("%s%s", prefixThumb, fileName)))
	// cloud storage
	bucket, err := storageClient.Bucket(event.Bucket)
	if err != nil {
		return fmt.Errorf("client.Bucket: %v", err)
	}
	ctx, cancelFunc := context.WithTimeout(ctx, time.Millisecond*10000)
	defer cancelFunc()

	srcFile := bucket.Object(filePath)
	thumbFile := bucket.Object(thumbFilePath)

	// make thumbnail
	err = applyImageMagick(ctx, srcFile, thumbFile, "-", "-auto-orient", "-thumbnail", fmt.Sprintf("%dx%d>", thumbMaxWidth, thumbMaxHeight), "-")
	if err != nil {
		return fmt.Errorf("applyImageMagick: %v", err)
	}

	err = srcFile.Delete(ctx)
	if err != nil {
		return fmt.Errorf("srcFile.Delete: %v", err)
	}

	attrsToUpdate := storage2.ObjectAttrsToUpdate{
		ContentType:   event.ContentType,
		CacheControl:  cacheControl,
		PredefinedACL: aclPublicRead,
	}
	_, err = thumbFile.Update(ctx, attrsToUpdate)
	if err != nil {
		return fmt.Errorf("thumbFile.Update: %v", err)
	}

	options := storage2.SignedURLOptions{
		GoogleAccessID: serviceAccountName,
		SignBytes: func(bytes []byte) ([]byte, error) {
			resp, err := iamService.Projects.ServiceAccounts.SignBlob(
				serviceAccountID,
				&iam.SignBlobRequest{BytesToSign: base64.StdEncoding.EncodeToString(bytes)},
			).Context(ctx).Do()
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(resp.Signature)
		},
		Method:  "GET",
		Expires: time.Now().AddDate(100, 0, 0),
	}

	thumbUrl, err := signedURLNormalized(thumbFile.BucketName(), thumbFile.ObjectName(), &options)
	if err != nil {
		return fmt.Errorf("signedURLNormalized: %v", err)
	}

	updates := []firestore.Update{
		{
			Path:  "imageUrl",
			Value: thumbUrl,
		},
	}
	return updateChannel(ctx, firestoreClient, fileDir, updates)
}

func signedURLNormalized(bucketName string, objectName string, options *storage2.SignedURLOptions) (string, error) {
	rawUrl, err := storage2.SignedURL(bucketName, objectName, options)
	if err != nil {
		return "", err
	}
	urlObj, err := url.Parse(rawUrl)
	if err != nil {
		return "", err
	}
	urlObj.RawQuery = ""
	return urlObj.String(), nil
}

func updateContact(ctx context.Context, firestoreClient *firestore.Client, fileDir string, updates []firestore.Update) error {
	contactPath := strings.ReplaceAll(fileDir, "/photo", "")
	_, err := firestoreClient.Doc(contactPath).Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("firestoreClient.Update: %v", err)
	}
	return nil
}

func updateChannel(ctx context.Context, firestoreClient *firestore.Client,
	fileDir string, updates []firestore.Update) error {
	channelPath := strings.ReplaceAll(fileDir, substrImages, "")
	_, err := firestoreClient.Doc(channelPath).Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("firestoreClient.Update: %v", err)
	}
	return nil
}

func updateUserProfile(ctx context.Context, authClient *auth.Client, firestoreClient *firestore.Client,
	filePath string, updates []firestore.Update) error {
	uid := strings.Split(filePath, "/")[1]
	_, err := firestoreClient.Collection(db.Users).Doc(uid).Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("firestoreClient.Update: %v", err)
	}
	userToUpdate := auth.UserToUpdate{}
	userToUpdate.PhotoURL(updates[1].Value.(string))
	_, err = authClient.UpdateUser(ctx, uid, &userToUpdate)
	if err != nil {
		return fmt.Errorf("authClient.UpdateUser: %v", err)
	}
	return nil
}

func applyImageMagick(ctx context.Context, srcFile *storage2.ObjectHandle, dstFile *storage2.ObjectHandle, cmdArgs ...string) error {
	reader, err := srcFile.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("file.NewReader: %v", err)
	}
	writer := dstFile.NewWriter(ctx)
	defer writer.Close()

	cmd := exec.Command("convert", cmdArgs...)
	cmd.Stdin = reader
	cmd.Stdout = writer

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("cmd.Run: %v", err)
	}

	log.Printf("applyed [%v]. image uploaded to gs://%s/%s", cmdArgs, dstFile.BucketName(), dstFile.ObjectName())
	return nil
}

func isChannelImage(filePath string) (result bool) {
	result = strings.Contains(filePath, substrChannels) && strings.Contains(filePath, substrImages)
	return
}
