package main

import (
	"fmt"
	"os/exec"
)

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := fmt.Sprintf("%s.processing", filePath)
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to process video for fast start - error: %v", err)
	}

	return outputPath, nil
}
