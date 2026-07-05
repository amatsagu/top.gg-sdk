# Top.gg Go SDK

<a href="https://pkg.go.dev/github.com/top-gg-community/go-sdk">
	<img src="https://pkg.go.dev/badge/github.com/top-gg-community/go-sdk.svg" alt="Go Reference">
</a>
<br><br>

The community-maintained Go SDK for Top.gg.
> For more information, see the documentation here: <https://docs.top.gg>.

## Chapters

- [Installation](#installation)
- [Setting up](#setting-up)
- [Usage](#usage)
  - [Getting your project's information](#getting-your-projects-information)
  - [Updating your project's information](#updating-your-projects-information)
  - [Getting your project's vote information of a user](#getting-your-projects-vote-information-of-a-user)
  - [Getting a paginated list of votes for your project](#getting-a-paginated-list-of-votes-for-your-project)
  - [Posting an announcement for your project](#posting-an-announcement-for-your-project)
  - [Posting your project's metric stats](#posting-your-projects-metric-stats)
  - [Posting your bot's application commands list](#posting-your-bots-application-commands-list)
  - [Webhooks](#webhooks)

## Installation

```sh
go get github.com/top-gg/go-dbl
```

## Setting up

```go
package main

import (
	"log"
	
	"github.com/top-gg/go-dbl"
)

func main() {
	client := dbl.NewClient(dbl.ClientOptions{
		Token: "YOUR_TOP_GG_TOKEN",
	})
}
```

## Usage

### Getting your project's information

```go
project, err := client.GetMyProject()
if err != nil {
	log.Fatal(err)
}

log.Printf("Project ID: %s, Name: %s", project.ID, project.Name)
```

### Updating your project's information

```go
err := client.EditMyProject(dbl.ProjectPayload{
	Headline: map[dbl.Locale]string{
		dbl.LocaleEnglish: "A great bot with tons of features!",
	},
	PageContent: map[dbl.Locale]string{
		dbl.LocaleEnglish: "# Welcome\nThis is the full page description for your project...",
	},
})
```

### Getting your project's vote information of a user

#### Discord ID

```go
vote, err := client.GetVote(661200758510977084, "discord")
```

### Getting a paginated list of votes for your project

```go
// Fetch votes starting from a specific date
since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
votes, err := client.GetVotes("", &since)

log.Printf("Fetched %d votes", len(votes.Votes))

// Fetch the next page using the cursor
nextPage, err := client.GetVotes(votes.Cursor, nil)
```

### Posting an announcement for your project 

```go
announcement, err := client.PostAnnouncement(
	"Version 2.0 Released!",
	"We just released version 2.0 with a bunch of new features and improvements.",
	"", // Category (optional)
)

log.Printf("Announcement posted at: %s", announcement.CreatedAt)
```

### Posting your project's metric stats

#### Single

```go
err := client.PostMyMetrics(dbl.MetricsPayload{
	ServerCount: 420,
	ShardCount:  53,
})
```

#### Batch

```go
err := client.PostMyMetricsInBatch([]dbl.MetricsPayload{
	{
		ServerCount: 420,
		ShardCount:  53,
	},
	{
		ServerCount: 435,
	},
})
```

### Posting your bot's application commands list

```go
// Assuming you have a JSON array of raw Discord application commands
var rawCommands []any

err := client.PostApplicationCommands(rawCommands)
```

### Webhooks

Use the unified webhook handler to easily handle both legacy v0 and modern v1 crypto webhooks using the standard `http.Handler` interface:

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/top-gg/go-dbl"
)

func main() {
	client := dbl.NewClient(dbl.ClientOptions{
		Token: "YOUR_TOP_GG_TOKEN",
	})

	webhookHandler := client.NewWebhookHandler(dbl.WebhookOptions{
		Secret: "YOUR_WEBHOOK_SECRET",
		OnVote: func(vote dbl.VoteCreatePayload) {
			fmt.Printf("Received vote from user %s with weight %d\n", vote.User.ID, vote.Weight)
		},
		OnIntegrationCreate: func(integration dbl.IntegrationCreatePayload) {
			fmt.Printf("Integration created! Auto-updated secret to: %s\n", integration.Secret)
		},
	})

	http.Handle("/webhook", webhookHandler)
	http.ListenAndServe(":8080", nil)
}
```
