// This is just a bunch of example code merged together

// This is based off of the example code at https://github.com/go-irc/irc/tree/v3.1.4#example
/* go-irc/irc license
Copyright 2016 Kaleb Elwert

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

// Plus the example code at https://developers.google.com/sheets/api/quickstart/go , unknown license

// Other code not derived from the above examples is licensed under CC0 1.0 Universal
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gopkg.in/irc.v3"
)

var (
	logger     *logrus.Logger
	newMessage chan *irc.Message
	newRedeem  chan *CustomRedeem
)

type Config struct {
	DebugLog bool

	IRCAddress string
	IRCNick    string
	IRCPass    string
	IRCUser    string
	IRCName    string
	IRCChannel string

	GoogleCredentialsPath        string
	GoogleUserTokenPath          string
	GoogleSpreadsheetID          string
	GoogleSpreadsheetAppendRange string
}

func (cfg *Config) SheetsClient(ctx context.Context) (*sheets.Service, error) {
	b, err := os.ReadFile(cfg.GoogleCredentialsPath)
	if err != nil {
		logger.WithError(err).WithField("path", cfg.GoogleCredentialsPath).Warn("failed to open cred path")
		return nil, err
	}

	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		logger.WithError(err).Warn("failed to build config from json")
		return nil, err
	}

	f, err := os.Open(cfg.GoogleUserTokenPath)
	if err != nil {
		logger.WithError(err).WithField("path", cfg.GoogleUserTokenPath).Warn("failed to open token path")
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	if err != nil {
		logger.WithError(err).WithField("path", cfg.GoogleUserTokenPath).Warn("failed to decode token")
		return nil, err
	}

	client := config.Client(ctx, tok)

	return sheets.NewService(ctx, option.WithHTTPClient(client))
}

func main() {
	ctx := context.Background()
	newMessage = make(chan *irc.Message, 32)
	newRedeem = make(chan *CustomRedeem, 32)
	logger = logrus.New()

	cfg := Config{
		DebugLog: os.Getenv("REDEEMLOG_LOGLEVEL") == "debug",

		IRCAddress: os.Getenv("REDEEMLOG_ADDRESS"),
		IRCNick:    os.Getenv("REDEEMLOG_NICK"),
		IRCPass:    os.Getenv("REDEEMLOG_PASS"),
		IRCUser:    os.Getenv("REDEEMLOG_USER"),
		IRCName:    os.Getenv("REDEEMLOG_NAME"),
		IRCChannel: os.Getenv("REDEEMLOG_CHANNEL"),

		GoogleCredentialsPath:        os.Getenv("REDEEMLOG_GOOGLE_CREDENTIALS_PATH"),         // read-only
		GoogleUserTokenPath:          os.Getenv("REDEEMLOG_GOOGLE_USER_TOKEN_PATH"),          // read/write
		GoogleSpreadsheetID:          os.Getenv("REDEEMLOG_GOOGLE_SPREADSHEET_ID"),           // part of the url
		GoogleSpreadsheetAppendRange: os.Getenv("REDEEMLOG_GOOGLE_SPREADSHEET_APPEND_RANGE"), // BotData!A:C
	}

	if cfg.DebugLog {
		logger.SetLevel(logrus.DebugLevel)
	}

	sheetsClient, err := cfg.SheetsClient(ctx)
	if err != nil {
		logger.WithError(err).Fatal("failed to boot sheets client, check config")
	}

	// process all IRC messages
	go func() {
		for msg := range newMessage {
			handleMessage(msg)
		}
	}()

	// process IRC messages that have been parsed into redeems
	go func() {
		for redeem := range newRedeem {
			resp, err := sheetsClient.Spreadsheets.Values.Append(
				cfg.GoogleSpreadsheetID,
				cfg.GoogleSpreadsheetAppendRange,
				&sheets.ValueRange{
					Values: [][]interface{}{
						{
							redeem.CustomRewardID,
							redeem.MillisecondEpochTS,
							redeem.NiceTime,
							redeem.MessageID,
							redeem.UserID,
							redeem.DisplayName,
							redeem.IRCName,
							redeem.Text,
						},
					},
				},
			).ValueInputOption("USER_ENTERED").Do()
			if err != nil {
				logger.WithError(err).WithField("redeem", redeem).Fatal("failed to append row")
			}
			logger.WithField("range", resp.Updates.UpdatedRange).Info("inserted row!")
		}
	}()

	conn, err := net.Dial("tcp", cfg.IRCAddress)
	if err != nil {
		logger.WithError(err).Fatal("failed to dial IRC address")
	}

	// received message counter
	ctr := int64(0)

	writeLoggedMessage := func(c *irc.Client, message string) error {
		err := c.Write(message)
		if err != nil {
			logger.WithError(err).WithField("message", message).Warn("failed to send message")
			return err
		}
		logger.WithField("message", message).Debug("sent message")
		return nil
	}

	config := irc.ClientConfig{
		Nick: cfg.IRCNick,
		Pass: cfg.IRCPass,
		User: cfg.IRCUser,
		Name: cfg.IRCName,
		Handler: irc.HandlerFunc(func(c *irc.Client, m *irc.Message) {
			logger.WithField("message", m.String()).Debug("received message")
			if m.Command == "001" {
				// 001 is a welcome event, so start our setup process
				writeLoggedMessage(c, "CAP REQ :twitch.tv/tags twitch.tv/commands")
				writeLoggedMessage(c, "JOIN #"+cfg.IRCChannel)
				logger.WithField("channel", cfg.IRCChannel).Info("knock knock")
			} else if m.Command == "ROOMSTATE" && m.Trailing() == "#"+cfg.IRCChannel {
				// emitted when a room join is successful
				logger.WithField("channel", cfg.IRCChannel).Info("party time")
			} else if m.Command == "PRIVMSG" && c.FromChannel(m) {
				// regular chat message
				newMessage <- m
				atomic.AddInt64(&ctr, 1)
			}
		}),
	}

	// print something after we receive our first message
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			if atomic.AddInt64(&ctr, 0) > 0 {
				logger.WithField("channel", cfg.IRCChannel).Info("received first regular chat message")
				return
			}
		}
	}()

	// print message velocity every x mins, unless 0
	go func() {
		ticker := time.NewTicker(time.Minute * 15)
		defer ticker.Stop()

		for {
			<-ticker.C
			cCtr := atomic.AddInt64(&ctr, 0)
			if cCtr == 0 {
				continue
			}
			atomic.AddInt64(&ctr, -cCtr)
			logger.WithField("messages", cCtr).Info("past 15 min")
		}
	}()

	// Create the client
	logger.Info("connecting")
	client := irc.NewClient(conn, config)
	err = client.Run()
	if err != nil {
		logger.WithError(err).Fatal("failure during IRC run")
	}
}

type CustomRedeem struct {
	CustomRewardID     string
	MessageID          string
	UserID             string
	DisplayName        string
	IRCName            string
	MillisecondEpochTS string
	NiceTime           string
	Text               string
}

func handleMessage(m *irc.Message) {
	var (
		customRedeem CustomRedeem
		ok           bool
	)
	customRedeem.CustomRewardID, ok = m.Tags.GetTag("custom-reward-id")
	if !ok {
		return // not a custom reward message
	}

	customRedeem.MessageID, _ = m.Tags.GetTag("id")
	customRedeem.UserID, _ = m.Tags.GetTag("user-id")
	customRedeem.DisplayName, _ = m.Tags.GetTag("display-name")
	customRedeem.IRCName = m.Name

	customRedeem.MillisecondEpochTS, ok = m.Tags.GetTag("tmi-sent-ts")
	if !ok {
		// default to current time if not found, but in same format
		customRedeem.MillisecondEpochTS = fmt.Sprintf("%v", time.Now().UnixMilli())
	}
	tsInt, err := strconv.ParseInt(customRedeem.MillisecondEpochTS, 10, 64)
	if err != nil {
		logger.WithError(err).WithField("ts", customRedeem.MillisecondEpochTS).Warn("failed to parse TS, ignoring")
	} else {
		customRedeem.NiceTime = time.UnixMilli(tsInt).Format(time.RFC3339)
	}

	customRedeem.Text = m.Trailing()

	logger.WithField("custom-redeem", customRedeem).Info("found custom redeem")
	newRedeem <- &customRedeem
}
