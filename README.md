# Gimertificator
Gimertificator is tool for remind about giltab merge requests to slack

## Install
```shell script
go get -u github.com/vortgo/gimertificator
```
## Preconditions

* Gitlab [access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)  
* Slack [webhook url](https://api.slack.com/messaging/webhooks)

## Show run options

```shell script
gimertificator -h
```

## Usage

```shell script
gimertificator  -token={your gitlab token} -slackWebhookUrl={yout slack webhook url}
```
for self-hosted gitlab
```shell script
gimertificator -gitlabApiUrl=https://{your gitlab domain}/api/v4 -token={your gitlab token} -slackWebhookUrl={yout slack webhook url}
```