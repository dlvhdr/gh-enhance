package tui

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	log "github.com/charmbracelet/log/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	gh "github.com/cli/go-gh/v2/pkg/api"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

func TestFullOutput(t *testing.T) {
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.SetOutput(os.Stdout)
		log.SetLevel(log.DebugLevel)
		log.SetReportCaller(true)
		log.SetTimeFormat(time.Kitchen)
		log.SetColorProfile(colorprofile.TrueColor)
	}
	setMockClient(t)

	m := NewModel("dlvhdr/gh-enhance", "1", ModelOpts{})
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 60))

	waitForText(t, tm, "fix(prompt): prompt mark not placed after text edits correctly", teatest.WithDuration(5*time.Second))
	tm.Send(tea.KeyPressMsg{
		Text: "ctrl+c",
	})

	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	fm := tm.FinalModel(t).(model)
	fv := fm.View()

	if !strings.Contains(fv, "lintcommit") {
		t.Errorf(`couldn't find "lintcommit" run`)
	}
}

// localRoundTripper is an http.RoundTripper that executes HTTP transactions
// by using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func mustRead(t *testing.T, r io.Reader) string {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mustWrite(t *testing.T, w io.Writer, s string) {
	t.Helper()
	_, err := io.WriteString(w, s)
	if err != nil {
		panic(err)
	}
}

func setMockClient(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Debug("got http request", "url", req.URL.String(), "method", req.Method)
		switch {
		// https://api.github.com/repos/dlvhdr/gh-enhance/actions/jobs/44932094595
		case req.Method == http.MethodGet && strings.Contains(req.URL.String(), "/actions/jobs/"):
			mustWrite(t, w, "")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/graphql", func(w http.ResponseWriter, req *http.Request) {
		log.Debug("got graphql request", "url", req.URL.String(), "method", req.Method)
		body := ""
		if req.Method == http.MethodPost {
			body = mustRead(t, req.Body)
		}
		switch {
		case strings.Contains(body, "query FetchPR"):
			d, err := os.ReadFile("./testdata/fetchPR.json")
			if err != nil {
				t.Errorf("failed reading mock data file %v", err)
			}
			mustWrite(t, w, string(d))
		case strings.Contains(body, "query FetchCheckRuns"):
			d, err := os.ReadFile("./testdata/fetchCheckRuns.json")
			if err != nil {
				t.Errorf("failed reading mock data file %v", err)
			}
			mustWrite(t, w, string(d))
		case strings.Contains(body, "query FetchCheckRunSteps"):
			d, err := os.ReadFile("./testdata/fetchCheckRunSteps.json")
			if err != nil {
				t.Errorf("failed reading mock data file %v", err)
			}
			mustWrite(t, w, string(d))
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
	})
	client, err := gh.NewGraphQLClient(gh.ClientOptions{
		Transport: localRoundTripper{handler: mux},
		Host:      "localhost:3000",
		AuthToken: "fake-token",
	})
	if err != nil {
		t.Errorf("failed creating gh client %v", err)
	}
	api.SetClient(client)
}

func bytesContains(t *testing.T, bts []byte, str string) bool {
	t.Helper()
	return bytes.Contains(bts, []byte(str))
}

func waitForText(t *testing.T, tm *teatest.TestModel, text string, options ...teatest.WaitForOption) {
	teatest.WaitFor(t,
		tm.Output(),
		func(bts []byte) bool {
			contains := bytesContains(t, bts, text)
			if _, debug := os.LookupEnv("DEBUG"); debug {
				if contains {
					f, _ := os.CreateTemp("", "gh-enhance-debug")
					defer f.Close()
					fmt.Fprintf(f, "%s", string(bts))
					log.Debug("✅ wrote to file while looking for text", "file", f.Name(), "text", text)
				} else {
					f, _ := os.CreateTemp("", "not-found-gh-enhance-debug")
					defer f.Close()
					fmt.Fprintf(f, "%s", string(bts))
					log.Debug("❌ text not found", "file", f.Name(), "text", text)
				}
			}
			return contains
		},
		options...,
	)
}
