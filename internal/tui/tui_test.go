package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

func TestFullOutput(t *testing.T) {
	setupLogger(t)

	m := NewModel(ModelOpts{Repo: "dlvhdr/gh-enhance", PRNumber: "1"})
	m.client = makeMockClient(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 60))

	waitForText(
		t,
		tm,
		"",
		"fix(prompt): prompt mark not placed after text edits correctly",
		teatest.WithDuration(5*time.Second),
	)
	tm.Send(tea.KeyPressMsg{
		Text: "ctrl+c",
	})

	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	fm := tm.FinalModel(t).(model)
	fv := fm.View().Content

	if !strings.Contains(fv, "lintcommit") {
		t.Errorf(`couldn't find "lintcommit" run`)
	}
}
