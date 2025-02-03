package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find video", err)
		return
	}

	if userID != dbVideo.UserID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized for this action", nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Unable to close file for video", err)
			return
		}
	}(file)

	contentTypeHeader := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Unsupported Media Type", nil)
	}

	tempFile, err := os.CreateTemp("", "tubely-upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err = io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to copy file contents", err)
		return
	}

	if _, err = tempFile.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to reset file pointer", err)
		return
	}

	bytesForName := 32
	b := make([]byte, bytesForName)
	_, err = rand.Read(b)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating name for file", err)
		return
	}

	randomFileName := base64.RawURLEncoding.EncodeToString(b)

	videoAspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video aspect ratio", err)
		return
	}

	processedVideoPath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing video", err)
		return
	}
	defer os.Remove(processedVideoPath)

	processedFile, err := os.Open(processedVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to open processed file", err)
		return
	}
	defer processedFile.Close()

	videoKey := fmt.Sprintf("%s/%s.%s", videoAspectRatio, randomFileName, "mp4")

	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("S3_BUCKET")),
		Key:         aws.String(videoKey),
		Body:        processedFile,
		ContentType: aws.String(mediaType),
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to upload video", err)
		return
	}

	videoS3URL := fmt.Sprintf("%s,%s", os.Getenv("S3_BUCKET"), videoKey)
	dbVideo.VideoURL = &videoS3URL
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video", err)
		return
	}

	presignedVideo, err := cfg.dbVideoToSignedVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to sign video", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, presignedVideo)
}
