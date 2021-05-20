package main

import (
	"bytes"
	"flag"
	"github.com/AlekSi/pointer"
	"github.com/slack-go/slack"
	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"strconv"
	"text/template"
	"time"
)

const (
	page    = 1
	perPage = 1000
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
		mrTimeoutInHours int
		slackApiToken    string
		configFilePath   string
	)
	flag.StringVar(&token, "token", "", "Token for gitlab api")
	flag.StringVar(&gitlabApiUrl, "gitlabApiUrl", "https://gitlab.com/api/v4", "Gitlab api url")
	flag.StringVar(&slackApiToken, "slackApiToken", "", "Slack Api Token")
	flag.StringVar(&configFilePath, "configFilePath", "./config.yml", "Config File Path")
	flag.IntVar(&mrTimeoutInHours, "mrTimeoutInHours", 72, "Timeout for merge requests in hours")
	flag.Parse()

	if len(token) == 0 {
		log.Fatal("Set a valid token via run options")
	}
	if len(slackApiToken) == 0 {
		log.Fatal("Set a valid slackApiToken via run options")
	}

	gitClient, err := gitlab.NewClient(token, gitlab.WithBaseURL(gitlabApiUrl))
	if err != nil {
		log.Fatalf("Failed to create gitlab api client: %v", err)
	}

	mrs := getMergeRequest(err, gitClient)
	if err != nil {
		log.Fatalf("Failed to get list of merge reqursts: %v", err)
	}

	var mrsForNotify = filterMrForNotify(mrs, mrTimeoutInHours)
	if len(mrsForNotify) == 0 {
		log.Println("There are not merge requests for notify")
		return
	}

	mergeRequestsForUsers := shareMergeRequestsAmongUsers(gitClient, mrsForNotify)

	var mapOfUsers map[string]string

	fileData, _ := ioutil.ReadFile(configFilePath)
	if err := yaml.Unmarshal(fileData, &mapOfUsers); err != nil {
		log.Panic(err)
	}

	api := slack.New(slackApiToken)

	for gitlabUsername, userNotifyId := range mapOfUsers {
		if _, ok := mergeRequestsForUsers[gitlabUsername]; !ok {
			continue
		}

		message := createNotifyMessage(mergeRequestsForUsers[gitlabUsername])

		_, _, err = api.PostMessage(userNotifyId, slack.MsgOptionText(message, false))
		if err != nil {
			log.Fatalf("Failed to send request to slack: %v", err)
		}
	}
}

func createNotifyMessage(mergeRequests []*gitlab.MergeRequest) string {
	data := NotificationTemplateData{
		MrsCount:      len(mergeRequests),
		MergeRequests: mergeRequests,
	}

	funcMap := template.FuncMap{
		"dateDelta": func(createdAt time.Time) string {
			deltaDuration := time.Now().Sub(createdAt)
			return strconv.Itoa(int(deltaDuration.Hours() / 24))
		},
	}

	t, err := template.New("notification").Funcs(funcMap).Parse(`
{{ .MrsCount }} merge requests waiting for your approval
{{range .MergeRequests}}
<{{.WebURL}}|{{.Title}}> ({{.Author.Username}}) - {{.CreatedAt|dateDelta}} days {{end}}`)

	if err != nil {
		log.Fatalf("Failed to load template for notification: %v", err)
	}

	var writer bytes.Buffer
	if err := t.Execute(&writer, data); err != nil {
		log.Fatalf("Failed to render template for notification: %v", err)
	}
	return writer.String()
}

func getMergeRequest(err error, git *gitlab.Client) []*gitlab.MergeRequest {
	opt := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{Page: page, PerPage: perPage},
		State:       pointer.ToString("opened"),
		OrderBy:     pointer.ToString("created_at"),
		Scope:       pointer.ToString("all"),
		WIP:         pointer.ToString("no"),
	}

	mrs, _, err := git.MergeRequests.ListMergeRequests(opt)
	return mrs
}

func filterMrForNotify(mrs []*gitlab.MergeRequest, mrTimeoutInHours int) []*gitlab.MergeRequest {
	var mrForNotify []*gitlab.MergeRequest
	for _, mr := range mrs {
		deltaDuration := time.Now().Sub(pointer.GetTime(mr.CreatedAt))
		if deltaDuration.Hours() > float64(mrTimeoutInHours) {
			mrForNotify = append(mrForNotify, mr)
		}
	}

	return mrForNotify
}

func shareMergeRequestsAmongUsers(git *gitlab.Client, mrs []*gitlab.MergeRequest) map[string][]*gitlab.MergeRequest {
	var usersMergeRequests = make(map[string][]*gitlab.MergeRequest)

	for _, currentMr := range mrs {
		approvers, _, err := git.MergeRequests.GetMergeRequestApprovals(currentMr.ProjectID, currentMr.IID)

		if err != nil {
			log.Fatalf("Failed to get merge request approvals: %v", err)
		}

		if approvers == nil {
			continue
		}

		reviewers := getReviewersForMergeRequest(currentMr)
		needReviewFromUsers := getUsersNoReviewYet(reviewers, approvers.ApprovedBy)

		for _, username := range needReviewFromUsers {
			if slice, ok := usersMergeRequests[username]; ok {
				usersMergeRequests[username] = append(slice, currentMr)
			} else {
				usersMergeRequests[username] = []*gitlab.MergeRequest{currentMr}
			}
		}
	}
	return usersMergeRequests
}

func getReviewersForMergeRequest(mr *gitlab.MergeRequest) []string {
	var reviewers []string

	for _, reviewer := range mr.Reviewers {
		reviewers = append(reviewers, reviewer.Username)
	}

	return reviewers
}

func getUsersNoReviewYet(reviewers []string, approvedBy []*gitlab.MergeRequestApproverUser) []string {
	for i := range approvedBy {
		reviewers = deleteElementFromSliceIfExists(reviewers, approvedBy[i].User.Username)
	}

	return reviewers
}

func deleteElementFromSliceIfExists(slice []string, value string) []string {
	for i := range slice {
		if slice[i] == value {
			slice[i] = slice[len(slice)-1]
			newSlice := slice[:len(slice)-1]
			return newSlice
		}
	}

	return slice
}
