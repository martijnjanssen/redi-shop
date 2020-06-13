package util

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

var (
	CHANNEL_ORDER   = "CHAN_ORDER"
	CHANNEL_PAYMENT = "CHAN_PAYMENT"
	CHANNEL_STOCK   = "CHAN_STOCK"

	// Payment events
	MESSAGE_PAY        = "MESG_PAY"
	MESSAGE_PAY_REVERT = "MESG_PAY_REV"

	// Stock events
	MESSAGE_STOCK = "MESG_STOCK"

	// Order request response events
	MESSAGE_ORDER_SUCCESS    = "MESG_ORDER_SUCCESS"
	MESSAGE_ORDER_BADREQUEST = "MESG_ORDER_BAD"
	MESSAGE_ORDER_INTERNAL   = "MESG_ORDER_INTERNAL"

	// Error types to determine the response
	INTERNAL_ERR = errors.New("INTERNAL_ERR")
	BAD_REQUEST  = errors.New("BAD_REQUEST")
)

func SetupSubChannel(ctx context.Context, r *redis.Client, channel string) string {
	channelID := uuid.Must(uuid.NewV4()).String()

	go func() {
		res := r.SAdd(ctx, channel, channelID)
		if res.Val() != 1 {
			logrus.Panic("unable to listen on new channel")
		}
	}()

	return fmt.Sprintf("%s.%s", channel, channelID)
}

// Publishes to ONE random listening client
func Pub(r *redis.Client, ctx context.Context, channel string, trackID string, message string, payload string) {
	get := r.SRandMember(ctx, channel)
	if get.Err() != nil {
		logrus.WithField("channel", channel).Error("unable to get client to send to")
		return
	}

	err := r.Publish(ctx, fmt.Sprintf("%s.%s", channel, get.Val()), fmt.Sprintf("%s#%s#%s", trackID, message, payload)).Err()
	if err != nil {
		logrus.WithField("channel", channel).WithField("messsage", message).WithError(err).Error("unable to publish message")
	}
}

func HTTPErrorToSAGAError(status int) error {
	if status == fasthttp.StatusOK {
		return nil
	} else if status == fasthttp.StatusInternalServerError {
		return INTERNAL_ERR
	}

	return BAD_REQUEST
}
