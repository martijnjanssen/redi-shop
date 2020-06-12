package util

import (
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type ConnectionType int

const (
	POSTGRES ConnectionType = 1
	REDIS    ConnectionType = 2
)

func GetConnectionType(name string) ConnectionType {
	switch name {
	case "postgres":
		return POSTGRES
	case "redis":
		return REDIS
	default:
		logrus.WithField("backend", name).Fatal("invalid backend, should be one of: postgres, redis")
		return 0
	}
}

// Connection struct to pass into the service
type Connection struct {
	Backend  ConnectionType
	Postgres *gorm.DB
	Redis    *redis.Client

	Broker *redis.Client
	URL    Services
}

type Services struct {
	User    string
	Order   string
	Stock   string
	Payment string
}
