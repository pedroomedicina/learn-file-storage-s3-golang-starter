package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

type FFProbeStream struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type FFProbeOutput struct {
	Streams []FFProbeStream `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run ffprobe - error: %v", err)
	}

	var probeOutput FFProbeOutput
	err := json.Unmarshal(output.Bytes(), &probeOutput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal ffprobe - error: %v", err)
	}

	if len(probeOutput.Streams) == 0 {
		return "", errors.New("ffprobe returned no streams")
	}

	width := probeOutput.Streams[0].Width
	height := probeOutput.Streams[0].Height
	if width == 0 || height == 0 {
		return "", errors.New("invalid width or height")
	}

	ratio := float64(width) / float64(height)
	if ratio > 1.7 && ratio < 1.8 {
		return "landscape", nil
	} else if ratio > 0.55 && ratio < 0.57 {
		return "portrait", nil
	}

	return "other", nil
}
