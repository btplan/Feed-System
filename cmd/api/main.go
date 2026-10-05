package main

import (
	"log"

	"clipflow/internal/account"
	"clipflow/internal/auth"
	"clipflow/internal/config"
	"clipflow/internal/database"
	"clipflow/internal/httpapi"
	"clipflow/internal/video"
)

// main 创建 Gin 服务，注册健康检查地址并开始监听本机端口。
func main() {
	cfg, err := config.Load("configs/local.yaml")
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if err := account.Migrate(db); err != nil {
		log.Fatal(err)
	}
	if err := video.Migrate(db); err != nil {
		log.Fatal(err)
	}

	// 抽象TokenManager 登录签发、验证token
	tokens := auth.NewTokenManager(cfg.Auth.JWTSecret)

	accounts := account.NewHandler(account.NewService(account.NewRepository(db), tokens))
	videos := video.NewHandler(video.NewService(video.NewRepository(db), cfg.Storage.UploadDir))
	router := httpapi.NewRouter(tokens, accounts, videos)

	log.Fatal(router.Run(cfg.Server.Addr))
}
