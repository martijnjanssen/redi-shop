package util

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

var (
	CHANNEL_ORDER = "CHAN_ORDER"

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

func PubToOrder(r *redis.Client, ctx context.Context, orderChannelID string, trackID string, message string) {
	err := r.Publish(ctx, fmt.Sprintf("%s.%s", CHANNEL_ORDER, orderChannelID), fmt.Sprintf("%s#%s#%s#", orderChannelID, trackID, message)).Err()
	if err != nil {
		logrus.WithField("messsage", message).WithError(err).Error("unable to publish message")
	}
}

// Publishes to a running microservice
func Pub(url string, service string, orderChannelID string, trackID string, message string, payload string) {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(fmt.Sprintf("%s/%s/message", url, service))
	req.Header.SetMethod("POST")
	req.SetBodyString(fmt.Sprintf("%s#%s#%s#%s", orderChannelID, trackID, message, payload))

	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{}
	err := client.Do(req, resp)
	if err != nil {
		logrus.WithField("service", service).WithField("messsage", message).WithError(err).Error("unable to send message")
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		logrus.WithField("status", resp.StatusCode).Error("error while making request")
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
