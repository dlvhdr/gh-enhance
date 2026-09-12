package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	gh "github.com/cli/go-gh/v2/pkg/api"
	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/tui"
)

func main() {
	client := makeFakeClient()
	newConfigFile, fileErr := os.OpenFile("debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
	if fileErr != nil {
		panic(fileErr)
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(wd) // for example /home/user
	fmt.Print("logging to", path.Join(wd, newConfigFile.Name()))

	log.SetLevel(log.DebugLevel)
	log.SetColorProfile(colorprofile.TrueColor)
	log.SetOutput(newConfigFile)
	log.SetTimeFormat("15:04:05.000")
	log.SetReportCaller(true)
	log.Debug("Logging to debug.log")

	m := tui.NewModel(
		tui.ModelOpts{
			API:              client,
			Repo:             "neovim/neovim",
			PRNumber:         "1234",
			SmallScreenWidth: 0,
			Flat:             true,
		},
	)

	p := tea.NewProgram(m, tea.WithWindowSize(179, 40))
	if _, err := p.Run(); err != nil {
		log.Error("failed starting program", "err", err)
		fmt.Println(err)
		os.Exit(1)
	}
}

func makeFakeClient() api.API {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Debug("got http request", "url", req.URL.String(), "method", req.Method)
		switch {
		// https://api.github.com/repos/dlvhdr/gh-enhance/actions/jobs/44932094595
		case req.Method == http.MethodGet && strings.Contains(req.URL.String(), "/actions/jobs/"):
			d, err := os.ReadFile("./testdata/fetchJobSteps.json")
			if err != nil {
				panic(err)
			}
			mustWrite(w, string(d))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	page := 1
	mux.HandleFunc("/api/graphql", func(w http.ResponseWriter, req *http.Request) {
		log.Debug("got graphql request", "url", req.URL.String(), "method", req.Method)
		body := ""
		time.Sleep(200 * time.Millisecond)
		if req.Method == http.MethodPost {
			body = mustRead(req.Body)
		}
		switch {
		case strings.Contains(body, "query FetchPR"):
			d, err := os.ReadFile("../fetchPR.json")
			if err != nil {
				panic(err)
			}
			mustWrite(w, string(d))
		case strings.Contains(body, "query FetchCheckRuns"):
			d, err := os.ReadFile(fmt.Sprintf("./testdata/fetchCheckRunsPage%d.json", page))
			if page == 1 {
				page = 2
			}
			if err != nil {
				panic(err)
			}

			mustWrite(w, string(d))
		case strings.Contains(body, "query FetchCheckRunSteps"):
			d, err := os.ReadFile("../fetchCheckRunSteps.json")
			if err != nil {
				panic(err)
			}
			mustWrite(w, string(d))
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
	})
	gqlClient, err := gh.NewGraphQLClient(gh.ClientOptions{
		Transport: localRoundTripper{handler: mux},
		Host:      "localhost:3000",
		AuthToken: "fake-token",
	})
	if err != nil {
		panic(err)
	}
	httpClient, err := gh.NewHTTPClient(gh.ClientOptions{
		Transport: localRoundTripper{handler: mux},
		Host:      "localhost:3000",
		AuthToken: "fake-token",
	})
	if err != nil {
		panic(err)
	}

	tAPI := api.New()
	tAPI.SetGQLClient(gqlClient)
	tAPI.SetHTTPClient(httpClient)
	return tAPI
}

func mustRead(r io.Reader) string {
	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mustWrite(w io.Writer, s string) {
	_, err := io.WriteString(w, s)
	if err != nil {
		panic(err)
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
