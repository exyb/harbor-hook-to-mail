package utils

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	. "github.com/exyb/harbor-hook-to-mail/config"
)

var (
	cli    *client.Client
	config *RegistryConfig
)

func GetRegistryConfig() *RegistryConfig {
	if config != nil {
		return config
	}
	configPath := os.Getenv("config_file_path")
	config, _ = LoadRegistryConfig(configPath)
	encryptedPassword, err := base64.StdEncoding.DecodeString(config.Registry.Auth.Password)
	if err != nil {
		log.Fatalf("Failed to decode registry password from base64: %v", err)
	}

	decryptedPassword, err := DecryptAES(encryptedPassword)
	if err != nil {
		log.Fatalf("Failed to decode registry password: %v", err)
	}

	config.Registry.Auth.Password = string(decryptedPassword)

	return config
}

func GetClientInstance(ctx context.Context) (*client.Client, error) {
	if cli != nil {
		return cli, nil
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return cli, nil
}
func PullImage(imageName string) error {
	cli, _ := GetClientInstance(context.Background())
	config = GetRegistryConfig()
	authConfig := registry.AuthConfig{
		Username:      config.Registry.Auth.Username,
		Password:      config.Registry.Auth.Password,
		ServerAddress: config.Registry.Address,
	}
	authResponse, err := cli.RegistryLogin(context.Background(), authConfig)
	if err != nil {
		log.Fatalf("Error logging in to Harbor: %s", err)
	}
	defer cli.Close()

	fmt.Printf("Login successful: %s\n", authResponse.Status)

	// 创建 RegistryAuth
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		log.Fatalf("Error encoding auth config: %s", err)
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	reader, err := cli.ImagePull(context.Background(), imageName, types.ImagePullOptions{RegistryAuth: authStr})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// 输出拉取过程
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		log.Fatalf("Error reading image pull response: %s", err)
	}

	fmt.Println("Image pull completed successfully.")
	return nil
}

func runContainerInBackGround(imageName string) (string, error) {
	ctx := context.Background()
	cli, _ := GetClientInstance(context.Background())
	defer cli.Close()

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		// Cmd:        strslice.StrSlice([]string{"cat"}),
		Entrypoint: strslice.StrSlice([]string{"tail", "-f", "/dev/null"}),
		Tty:        false,
	}, &container.HostConfig{
		AutoRemove: true,
	}, nil, nil, "")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		panic(err)
	}

	return resp.ID, nil
}

func waitForContainer(ctx context.Context, cli *client.Client, containerID string) error {
	timeout := 60 * time.Second
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
			info, err := cli.ContainerInspect(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}

			switch info.State.Status {
			case "exited":
				return fmt.Errorf("container has exited with status %d", info.State.ExitCode)
			case "running":
				return nil
			}

			if time.Since(startTime) >= timeout {
				return fmt.Errorf("timeout while waiting for container")
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func copyFileFromContainer(ctx context.Context, cli *client.Client, containerID, containerPath, localPath string) error {
	reader, _, err := cli.CopyFromContainer(ctx, containerID, containerPath)
	if err != nil {
		return fmt.Errorf("failed to copy file from container: %w", err)
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)

	if _, err := os.Stat(localPath); err == nil {
		// clear file content
		file, err := os.OpenFile(localPath, os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Println("Error:", err)
		}
		defer file.Close()

		fmt.Println("File", localPath, "exists. Cleared content.")
	}

	dir := filepath.Dir(localPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating local directory: %w", err)
		}
		fmt.Println("Directory created:", dir)
	} else if err != nil {
		return fmt.Errorf("error checking local directory: %w", err)
	}

	destFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer destFile.Close()

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(localPath, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("failed to create local directory base on source file info: %w", err)
			}
			continue
		}

		file, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(file, tarReader); err != nil {
			return fmt.Errorf("failed to write file content: %w", err)
		}
	}

	fmt.Printf("File %s copied from container %s to %s\n", containerPath, containerID, localPath)
	return nil
}
func ExtractFileFromImage(imageName, containerPath, localPath string) error {
	ctx := context.Background()
	cli, _ := GetClientInstance(context.Background())
	defer cli.Close()

	containerID, err := runContainerInBackGround(imageName)
	fmt.Printf("Created container %s, image: %s\n", containerID, imageName)
	if err != nil {
		return fmt.Errorf("failed to create container with image %s: %w", imageName, err)
	}

	if err := waitForContainer(ctx, cli, containerID); err != nil {
		return fmt.Errorf("wait container error: %w", err)
	}

	defer func() {
		// always stop temp containers
		fmt.Printf("Stopping container %s, image: %s\n", containerID, imageName)
		noWaitTimeout := 0 // to not wait for the container to exit gracefully
		if err := cli.ContainerStop(ctx, containerID, containertypes.StopOptions{Timeout: &noWaitTimeout}); err != nil {
			panic(err)
		}
	}()

	// fmt.Println("container is ready, now list running containers")
	// containers, err := cli.ContainerList(ctx, containertypes.ListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// for _, container := range containers {
	// 	fmt.Println(container.ID, container.Image, container.ImageID)
	// }

	// copy file from container
	if err := copyFileFromContainer(ctx, cli, containerID, containerPath, localPath); err != nil {
		return fmt.Errorf("copy file error: %w", err)
	}

	return nil
}
