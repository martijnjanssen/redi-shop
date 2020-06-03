package server

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
)

var services = map[string]func(*util.Connection) fasthttp.RequestHandler{
	"user":    getUserRouter,
	"stock":   getStockRouter,
	"payment": getPaymentRouter,
	"order":   getOrderRouter,
}

// Start initializes the database connection and starts listening to incoming requests
func Start() {
	service := viper.GetString("service")

	// Connect to the correct backend
	conn := &util.Connection{Backend: util.GetConnectionType(viper.GetString("backend"))}
	switch conn.Backend {
	case util.POSTGRES:
		// Open database connection
		db, err := gorm.Open("postgres",
			fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
				viper.GetString("postgres.url"),
				viper.GetInt("postgres.port"),
				viper.GetString("postgres.database"),
				viper.GetString("postgres.username"),
				viper.GetString("postgres.password"),
			))
		if err != nil {
			logrus.WithError(err).Fatal("unable to connect to database")
		}
		defer func() {
			if err := db.Close(); err != nil {
				logrus.WithError(err).Error("unable to close database connection")
			}
		}()

		conn.Postgres = db

	case util.REDIS:
		client := redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%d", viper.GetString("redis.url"), viper.GetInt("redis.port")),
			// TODO: enable password access for redis
			// https://github.com/go-redis/redis/pull/1325
			// Password: viper.GetString("redis.password"),
			DB: 0, // use default DB
		})
		conn.Redis = client
	}

	// Get the handlerFunc for the service we want to use
	handlerFn, ok := services[service]
	if !ok {
		logrus.WithField("service", service).Fatal("service does not exist, valid services are: user, stock, order, payment")
	}

	// Start listening to incoming requests
	logrus.WithField("service", service).Info("Redi-shop started, awaiting requests...")
	err := fasthttp.ListenAndServe(":8000", handlerFn(conn))
	if err != nil {
		logrus.WithError(err).Fatal("error while listening")
	}
}
