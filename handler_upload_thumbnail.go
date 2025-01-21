package main

import (
	"encoding/base64"
	"fmt"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Unable to close file for thumbnail", err)
			return
		}
	}(file)

	mediaType := header.Header.Get("Content-Type")
	imageData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
		return
	}

	imageBase64Data := base64.StdEncoding.EncodeToString(imageData)

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find video", err)
		return
	}

	if userID != dbVideo.UserID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized for this action", nil)
		return
	}

	thumbnailUrl := fmt.Sprintf("data:%s;base64,%s", mediaType, imageBase64Data)
	dbVideo.ThumbnailURL = &thumbnailUrl
	dbVideo.UpdatedAt = time.Now()
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
