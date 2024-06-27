package handlers

import (
	"fmt"
	"path/filepath"

	. "github.com/exyb/harbor-hook-to-mail/utils"
)

func ImageHandler(namespace string, name string, tag string, resourceURL string) (string, []string, error) {
	var attachments []string
	attachments = make([]string, 0)
	localBuildLog := filepath.Join("/tmp", namespace, name, tag+".build.log")
	if err := GetFileFromImage(resourceURL, "/build.log", localBuildLog); err != nil {
		return "", nil, err
	}
	attachments = append(attachments, localBuildLog)

	localCommitLog := filepath.Join("/tmp", namespace, name, tag+".git_commit.txt")
	if err := GetFileFromImage(resourceURL, "/git_commit.txt", localCommitLog); err != nil {
		return "", nil, err
	}
	attachments = append(attachments, localCommitLog)

	localMailBody := filepath.Join("/tmp", namespace, name, tag+".mail.body")
	if err := GetFileFromImage(resourceURL, "/mail.body", localMailBody); err != nil {
		return "", nil, err
	}

	return localMailBody, attachments, nil
}

func GetFileFromImage(imageName string, containerFilePath string, localFilePath string) error {
	err := PullImage(imageName)
	if err != nil {
		fmt.Println("Failed to pull image:", err)
		return err
	}

	err = ExtractFileFromImage(imageName, containerFilePath, localFilePath)
	if err != nil {
		fmt.Println("Failed to extract file from image:", err)
		return err
	}

	fmt.Println("File extracted successfully")
	return nil
}
