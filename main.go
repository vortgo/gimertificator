package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/AlekSi/pointer"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"
)

const (
	page    = 1
	perPage = 1000

	slackResponseOk = "ok"
)

type NotificationTemplateData struct {
	MrsCount      int
	MergeRequests []*gitlab.MergeRequest
}

func main() {
	var (
		err              error
		token            string
		gitlabApiUrl     string
		slackWebhookUrl  string
		mrTimeoutInHours int
	)
	flag.StringVar(&token, "token", "", "Token for gitlab api")
	flag.StringVar(&gitlabApiUrl, "gitlabApiUrl", "https://gitlab.com/api/v4", "Gitlab api url")
	flag.StringVar(&slackWebhookUrl, "slackWebhookUrl", "", "Slack webhook URL for notification")
	flag.IntVar(&mrTimeoutInHours, "mrTimeoutInHours", 72, "Timeout for merge requests in hours")
	flag.Parse()

	if len(token) == 0 {
		log.Fatal("Set a valid token via run options")
	}
	if len(slackWebhookUrl) == 0 {
		log.Fatal("Set a valid slackWebhookUrl via run options")
	}

	git, err := gitlab.NewClient(token, gitlab.WithBaseURL(gitlabApiUrl))
	if err != nil {
		log.Fatalf("Failed to create gitlab api client: %v", err)
	}

	opt := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{Page: page, PerPage: perPage},
		State:       pointer.ToString("opened"),
		OrderBy:     pointer.ToString("updated_at"),
		Scope:       pointer.ToString("all"),
		WIP:         pointer.ToString("no"),
	}

	mrs, _, err := git.MergeRequests.ListMergeRequests(opt)
	if err != nil {
		log.Fatalf("Failed to get list of merge reqursts: %v", err)
	}

	var mrForNotify []*gitlab.MergeRequest
	for _, mr := range mrs {
		deltaDuration := time.Now().Sub(pointer.GetTime(mr.UpdatedAt))
		if deltaDuration.Hours() > float64(mrTimeoutInHours) {
			mrForNotify = append(mrForNotify, mr)
		}
	}

	if len(mrForNotify) == 0 {
		log.Println("There are not merge requests for notify")
		return
	}

	message := createNotifyMessage(mrForNotify)

	values := map[string]string{"text": message}
	jsonValue, err := json.Marshal(values)
	if err != nil {
		log.Fatal("Error occurred while make json body for notification request")
	}

	resp, err := http.Post(slackWebhookUrl, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Fatalf("Failed to make notification request: %v", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error occurred while read response from slack: %v", err)
	}

	if string(body) == slackResponseOk {
		log.Println(fmt.Sprintf("Syccessfully notificated about %d merge requests", len(mrForNotify)))
	}

}

func createNotifyMessage(mergeRequests []*gitlab.MergeRequest) string {
	data := NotificationTemplateData{
		MrsCount:      len(mergeRequests),
		MergeRequests: mergeRequests,
	}

	funcMap := template.FuncMap{
		"dateDelta": func(updatedAt time.Time) string {
			deltaDuration := time.Now().Sub(updatedAt)
			return strconv.Itoa(int(deltaDuration.Hours() / 24))
		},
	}

	t, err := template.New("notification").Funcs(funcMap).Parse(`
{{ .MrsCount }} merge requests waiting for your approval

{{range .MergeRequests}}
<{{.WebURL}}|{{.Title}}> ({{.Author.Username}}) - {{.UpdatedAt|dateDelta}} days {{end}}`)

	if err != nil {
		log.Fatalf("Failed to load template for notification: %v", err)
	}

	var writer bytes.Buffer
	if err := t.Execute(&writer, data); err != nil {
		log.Fatalf("Failed to render template for notification: %v", err)
	}
	return writer.String()
}
