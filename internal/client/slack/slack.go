package slack

import (
	"bytes"
	"delivery/internal/client/env"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"

	"delivery/internal/logger"
)

var log = logger.New(env.LogLevel)
var notifyTimeout int

func init() {
	var err error
	notifyTimeout, err = strconv.Atoi(env.NotifyTimeout)
	if err != nil {
		log.Fatalln("NOTIFY_DELIVERY_TIMEOUT must be of numeric type")
	}
}

func SendMessage(url *string, token *string, data *string) error {
	slackDefaultUrl := "https://slack.com/api/chat.postMessage"
	if url == nil {
		url = &slackDefaultUrl
	}

	buff := bytes.NewBuffer([]byte(*data))

	req, err := http.NewRequest("POST", *url, buff)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *token))

	client := &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout: time.Second * time.Duration(notifyTimeout),
		},
		Timeout: time.Second * time.Duration(notifyTimeout),
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return err
	}
	log.Debugln(string(b))
	return nil
}
