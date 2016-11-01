package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	raven "github.com/getsentry/raven-go"
	"github.com/julienschmidt/httprouter"
	"github.com/parnurzeal/gorequest"
)

var (
	// Sentry API Tokens can be obtained from: https://sentry.io/api/
	// (when using hosted Sentry). The minimum required scope is
	// "event:write".
	apiToken              = flag.String("api-token", "", "Sentry API Token")
	assignmentMappingsLoc = flag.String(
		"assign-loc",
		"./assignments.json",
		"JSON mapping of project to default assignee",
	)
	bindAddr = flag.String(
		"bind-addr",
		":18091",
		"Address to bind the web server to",
	)
	sentryDSN = flag.String("sentry-dsn", "", "Sentry DSN")

	// Users will provide a mapping of project slug -> username for the
	// default user to assign bugs to. As an example:
	//   {
	//     "sentry-bot": "name@domain.com",
	//     "sentry-bot-client": "name2@domain.com"
	//   }
	projectToUserMap = make(map[string]string)
)

func main() {
	flag.Parse()

	if !validStartup() {
		// We logged our error messages in validStartup
		return
	}

	mux := httprouter.New()
	attachHandlers(mux)
	logrus.Infof("Listening on %s...", *bindAddr)
	logrus.Error("Server exited: ", http.ListenAndServe(*bindAddr, mux))
}

// validStartup checks that we ahave all of the necessary information that
// we need in order to function. It logs any issues and returns true if we have
// all of the requisite information to run, false otherwise.
func validStartup() bool {
	if len(*sentryDSN) == 0 {
		logrus.Error("must provide a Sentry DSN")
		return false
	} else if len(*apiToken) == 0 {
		logrus.Error("must provide a Sentry API token")
		return false
	} else if f, err := os.Open(*assignmentMappingsLoc); err != nil {
		logrus.Error("no project -> user mapping: ", err)
		return false
	} else if err := json.NewDecoder(f).Decode(&projectToUserMap); err != nil {
		logrus.Error("failed to parse project -> user mapping: ", err)
		return false
	}

	raven.SetDSN(*sentryDSN)
	return true
}

// attachHandlers attaches all necessary handlers to the given mux.
func attachHandlers(mux *httprouter.Router) {
	mux.POST("/events", handleEvents)
	mux.POST("/fire", fireEvent)
}

// handleEvents handles an incoming webhook event from Sentry, parses the
// project name and ID, identifies the default user for the project, and then
// assigns the issue to them using the Sentry REST API.
//
// refs: https://docs.sentry.io/api/events/put-group-details/
//
// We always respond to all requests with 200 so that Sentry considers the
// webhook delivered.
func handleEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var i IssueInfo
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		logrus.Error("failed to read request body from Sentry: ", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	username, ok := projectToUserMap[i.Project]
	if !ok {
		logrus.Error("no default assignee for the %s project", username)
		w.WriteHeader(http.StatusOK)
		return
	}

	resp, _, errs := gorequest.New().
		Put(fmt.Sprintf("https://sentry.io/api/0/issues/%s/", i.ID)).
		Set("Authorization", fmt.Sprintf("Bearer %s", *apiToken)).
		Send(fmt.Sprintf(`{"assignedTo":"%s"}`, username)).
		End()
	if len(errs) != 0 {
		logrus.Error("failed to assign event to user:", errs)
		w.WriteHeader(http.StatusOK)
		return
	} else if resp.StatusCode != 200 {
		logrus.Error("received non-zero status from Sentry: ", resp.StatusCode)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Fire event fires a test event with the given culprit.
func fireEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var t TestEvent
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		logrus.Error("failed to decode test event information: ", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	logrus.Info("sending event to Sentry: ", t.Culprit)

	// TODO: support tags
	raven.CaptureMessage(t.Culprit, nil)
	w.WriteHeader(http.StatusOK)
}

// TestEvent contains the information we'd like to send in a test event to
// Sentry.
type TestEvent struct {
	Culprit string `json:"culprit"`
}

// IssueInfo is the information that comes as the payload of a webhook from
// Sentry. Currently we only decode issue ID and the project name slug.
type IssueInfo struct {
	ID      string `json:"id"`
	Project string `json:"project"`
}
