package api

import (
	"crypto/tls"
	"net/http"

	gh "github.com/cli/go-gh/v2/pkg/api"

	"github.com/charmbracelet/bubbletea-app-template/pkg/config"
)

func NewClient() (*gh.GraphQLClient, error) {
	if config.IsFeatureEnabled(config.FF_MOCK_DATA) {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		return gh.NewGraphQLClient(gh.ClientOptions{Host: "localhost:3000", AuthToken: "fake-token"})
	} else {
		return gh.DefaultGraphQLClient()
	}
}
