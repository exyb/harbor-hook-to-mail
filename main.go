package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

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
