package database

import (
	"time"

	"github.com/Laky-64/gologging"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"main/internal/utils"
)

var (
	client           *mongo.Client
	database         *mongo.Database
	settingsColl     *mongo.Collection
	chatSettingsColl *mongo.Collection

	// 🔹 المتغير العام المطلوب
	MongoDB *mongo.Database

	logger  = gologging.GetLogger("Database")
	dbCache = utils.NewCache[string, any](60 * time.Minute)
)

func Init(mongoURL string) func() {
	var err error
	logger.Debug("Initializing MongoDB...")
	client, err = mongo.Connect(options.Client().ApplyURI(mongoURL))
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB: %v", err)
	}

	logger.Debug("Successfully connected to MongoDB.")

	database = client.Database("YukkiMusic")
	MongoDB = database // 🔹 هنا نربط المتغير العام
	settingsColl = database.Collection("bot_settings")
	chatSettingsColl = database.Collection("chat_settings")

	go migrateData(mongoURL)

	return func() {
		ctx, cancel := mongoCtx()
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			logger.Error("Error while disconnecting MongoDB: %v", err)
		} else {
			logger.Info("MongoDB disconnected successfully")
		}
	}
}
