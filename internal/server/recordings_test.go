package server_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// The documentation plays recordings of what each view draws back to the
// real view page (mm-mcp#38). Each recording holds what a tool answered when
// it was made; it must decode into what that tool answers today, with no
// field the answer no longer has, so a change to what a view draws cannot
// leave the docs drawing what no tool answers. `task docs:record-views` makes
// them again.

// recordingsDir is where the documentation keeps the recordings.
var recordingsDir = filepath.Join("..", "..", "docs", "site", "assets", "views", "recordings")

// recordedAs is what each recording's call answered: by the recording's name,
// and by the tool and action each press it answers was.
var (
	recordedAs = map[string]func() any{
		"thread":   func() any { return &server.PostWithThread{} },
		"unreads":  func() any { return &server.Unreads{} },
		"posted":   func() any { return &server.Post{} },
		"proposal": func() any { return &server.Proposal{} },
		"confirm":  func() any { return &server.Proposal{} },
		"revision": func() any { return &server.Proposal{} },
		"choice":   func() any { return &server.Proposal{} },
		"preview":  func() any { return &server.Proposal{} },
	}
	pressedAs = map[string]func() any{
		"view_action:more":      func() any { return &server.PostWithThread{} },
		"view_action:unread":    func() any { return &server.UnreadPosts{} },
		"view_change:mark_read": func() any { return &server.Read{} },
		"view_change:post":      func() any { return &server.DraftChange{} },
		"view_change:replace":   func() any { return &server.DraftChange{} },
	}
)

// recordedResult is a tool's result as a recording holds it.
type recordedResult struct {
	Content           []json.RawMessage `json:"content"`
	StructuredContent json.RawMessage   `json:"structuredContent"`
}

func TestTheDocsRecordingsAreWhatTheToolsAnswer(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join(recordingsDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		found[name] = true
		make, ok := recordedAs[name]
		if !ok {
			t.Errorf("%s is a recording no test knows the answer of", file)
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var recording struct {
			Input     map[string]any              `json:"input"`
			Result    recordedResult              `json:"result"`
			Resources map[string]json.RawMessage  `json:"resources"`
			Answers   map[string][]recordedResult `json:"answers"`
		}
		if err := strictly(data, &recording); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := strictly(recording.Result.StructuredContent, make()); err != nil {
			t.Errorf("%s answers what its tool no longer does: %v", name, err)
		}
		for press, answers := range recording.Answers {
			make, ok := pressedAs[press]
			if !ok {
				t.Errorf("%s answers %s, a press no test knows the answer of", name, press)
				continue
			}
			for _, answer := range answers {
				if err := strictly(answer.StructuredContent, make()); err != nil {
					t.Errorf("%s's %s answers what the tool no longer does: %v", name, press, err)
				}
			}
		}
		for address := range recording.Resources {
			if !strings.HasPrefix(address, "mattermost://chat.example.com/") {
				t.Errorf("%s reads %s, which is not the docs' server", name, address)
			}
		}
		if strings.Contains(string(data), "lt-") || strings.Contains(string(data), "localhost") {
			t.Errorf("%s names the test stack it was recorded on", name)
		}
	}
	for name := range recordedAs {
		if !found[name] {
			t.Errorf("the recording %s is missing; task docs:record-views makes it", name)
		}
	}
}

// strictly decodes data into out, refusing a field out does not have.
func strictly(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}
