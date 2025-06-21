package parser

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

func TestBasicParseJobLogs(t *testing.T) {
	logs, err := os.ReadFile("./testdata/basic_logs.log")
	if err != nil {
		t.Error("failed opening the ./testdata/basci_logs.log file")
		return
	}
	steps, err := os.ReadFile("./testdata/expected_basic_logs.log")
	if err != nil {
		t.Error("failed opening the ./testdata/expected_basic_logs.log file")
		return
	}

	expectedSteps := make([]api.StepLogs, 0)
	for group := range strings.SplitSeq(string(steps), "----\n") {
		expectedSteps = append(expectedSteps, api.StepLogs(group))
	}

	assert.EqualValues(t, expectedSteps, GroupStepsLogsByMarkers("some-id", string(logs)))
}

func TestParseJobLogs(t *testing.T) {
	logs, err := os.ReadFile("./testdata/logs.log")
	if err != nil {
		t.Error("failed opening the ./testdata/logs.log file")
		return
	}
	steps, err := os.ReadFile("./testdata/expected_logs.log")
	if err != nil {
		t.Error("failed opening the ./testdata/expected_logs.log file")
		return
	}

	expectedSteps := make([]api.StepLogs, 0)
	for group := range strings.SplitSeq(string(steps), "----\n") {
		expectedSteps = append(expectedSteps, api.StepLogs(group))
	}

	assert.EqualValues(t, expectedSteps, GroupStepsLogsByMarkers("some-id", string(logs)))
}
