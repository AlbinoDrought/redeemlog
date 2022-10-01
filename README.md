# RedeemLog for Twitch

When someone redeems a chat-based channel point reward, append it to a Google Sheets spreadsheet.

This project uses Twitch's IRC "API" to avoid the cumbersome authentication requirements of the more appropriate PubSub / EventsAPI alternatives. Because of this, only chat-based redemptions will be caught (other redemptions do not create a chat message, do not appear over IRC "API")

## Configuration

| Env Var                                   | Description                                                                                                                                                                                 |
|-------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| REDEEMLOG_LOGLEVEL                        | Set to "debug" to see all sent/received IRC commands                                                                                                                                        |
| REDEEMLOG_ADDRESS                         | IRC address to connect to: you probably want `irc.chat.twitch.tv:6667`                                                                                                                      |
| REDEEMLOG_NICK                            | IRC nick                                                                                                                                                                                    |
| REDEEMLOG_USER                            | IRC user                                                                                                                                                                                    |
| REDEEMLOG_PASS                            | IRC pass                                                                                                                                                                                    |
| REDEEMLOG_NAME                            | IRC name                                                                                                                                                                                    |
| REDEEMLOG_CHANNEL                         | IRC channel: you probably want the name of your Twitch channel                                                                                                                              |
| REDEEMLOG_GOOGLE_CREDENTIALS_PATH         | Path to `credentials.json` downloaded from Google Cloud Console, containing a web client_id and client_secret, see Google demo https://developers.google.com/sheets/api/quickstart/go       |
| REDEEMLOG_GOOGLE_USER_TOKEN_PATH          | Path to `token.json` generated from an OAuth flow that has the `https://www.googleapis.com/auth/spreadsheets` scope, see Google demo https://developers.google.com/sheets/api/quickstart/go |
| REDEEMLOG_GOOGLE_SPREADSHEET_ID           | Spreadsheet ID, part of the URL when viewing Google Sheet in WebUI                                                                                                                          |
| REDEEMLOG_GOOGLE_SPREADSHEET_APPEND_RANGE | Where the data should be inserted, like `BotData!A:H`. Must be 8 columns wide.                                                                                                              |

## Running

`go get && go build && YOUR_CONFIG=here ./redeemlog`

or

`docker run --rm -e YOUR_CONFIG=here ghcr.io/albinodrought/redeemlog:latest`

(for fresh build: `docker build -t ghcr.io/albinodrought/redeemlog:latest .`)
