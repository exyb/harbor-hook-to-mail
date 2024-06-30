package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/exyb/harbor-hook-to-mail/routes"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func main() {
	mainDir, _ := os.Getwd()
	os.Setenv("config_file_path", filepath.Join(mainDir, "config.yaml"))

	ctx := context.Background()
	ctx = context.WithValue(ctx, "work_path", mainDir)
	ctx = context.WithValue(ctx, "config_file_path", filepath.Join(mainDir, "config.yaml"))

	// 设置信号处理器
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("Received an interrupt, saving map to file...")
		if err := routes.SaveMapToFile(); err != nil {
			fmt.Printf("Error saving map to file: %v\n", err)
		}
		os.Exit(0)
	}()

	// 处理 web 请求
	r := gin.Default()
	routes.SetupRouter(r)

	r.Use(func(c *gin.Context) {
		c.Set("ctx", ctx)
		c.Next()
	})

	config := viper.New()
	config.SetConfigFile("config.yaml")
	config.SetConfigType("yaml")
	if err := config.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("config file not found")
		} else {
			log.Fatalln(err)
		}
	}

	port := ":" + config.GetString("server.port")

	r.Run(port)
}
