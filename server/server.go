package server

import (
	"context"
	"fmt"
	"time"

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

		err = db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error
		if err != nil {
			logrus.WithError(err).Error("unable to create extension")
		}

		conn.Postgres = db

	case util.REDIS:
		client := redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%d", viper.GetString("redis.url"), viper.GetInt("redis.port")),
			// TODO: enable password access for redis
			// https://github.com/go-redis/redis/pull/1325
			// Password: viper.GetString("redis.password"),
			DB: 0, // use default DB
		})
		err := client.Ping(context.Background()).Err()
		if err != nil {
			logrus.WithError(err).Error("invalid redis connection")
		}
		conn.Redis = client
	}

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", viper.GetString("broker.url"), viper.GetInt("broker.port")),
		// TODO: enable password access for redis
		// https://github.com/go-redis/redis/pull/1325
		// Password: viper.GetString("broker.password"),
		DB:       0, // use default DB
		PoolSize: 1000,
	})
	err := client.Ping(context.Background()).Err()
	if err != nil {
		logrus.WithError(err).Error("invalid message broker connection")
	}
	conn.Broker = client

	conn.URL.User = viper.GetString("url.user")
	conn.URL.Order = viper.GetString("url.order")
	conn.URL.Stock = viper.GetString("url.stock")
	conn.URL.Payment = viper.GetString("url.payment")

	// Get the handlerFunc for the service we want to use
	handlerFn, ok := services[service]
	if !ok {
		logrus.WithField("service", service).Fatal("service does not exist, valid services are: user, stock, order, payment")
	}

	// Start listening to incoming requests
	logrus.WithField("service", service).Info("Redi-shop started, awaiting requests...")
	server := fasthttp.Server{
		Concurrency:   256 * 1024,
		MaxConnsPerIP: 3 * 1024,
		IdleTimeout:   10 * time.Second,
		Handler:       handlerFn(conn),
	}
	err = server.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("port")))
	if err != nil {
		logrus.WithError(err).Fatal("error while listening")
	}
}
