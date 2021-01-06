{{ .MrsCount }} merge requests waiting for your approval

{{range .MergeRequests}}
<{{.WebURL}}|{{.Title}}> ({{.Author.Username}}) - {{.UpdatedAt|dateDelta}} days {{end}}