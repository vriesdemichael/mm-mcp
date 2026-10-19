package scriptedmodel

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The script, as a client's messages reach it.

func said(role, content string) message {
	raw, _ := json.Marshal(content)
	return message{Role: role, Content: raw}
}

func TestTheScriptCallsTheToolTheMessageNames(t *testing.T) {
	t.Parallel()
	offered := []string{"mattermost__read_post", "mattermost__show", "mcp_mm-mcp_get_me", "mcp_mm-mcp_read_channel"}
	for _, c := range []struct {
		messages   []message
		name, args string
		textSays   string
	}{
		{[]message{said("user", `SCRIPT show {"kind":"thread","post_id":"p1"}`)}, "mattermost__show", `{"kind":"thread","post_id":"p1"}`, ""},
		{[]message{said("user", "SCRIPT get_me")}, "mcp_mm-mcp_get_me", "{}", ""},
		{[]message{said("user", `<context>the workspace</context> SCRIPT read_post {"post_id":"p1"} and some trailing text`)}, "mattermost__read_post", `{"post_id":"p1"}`, ""},
		{[]message{said("user", "SCRIPT show {}"), said("assistant", ""), said("tool", "shown")}, "", "", Done},
		{[]message{said("user", "SCRIPT post {}")}, "", "", "no tool offered is post"},
		{[]message{said("user", "SCRIPT channel {}")}, "", "", "no tool offered is channel"},
		{[]message{said("user", "SCRIPT read_channel {}")}, "mcp_mm-mcp_read_channel", "{}", ""},
		{[]message{said("user", "SCRIPT show {not json")}, "", "", "not JSON"},
		{[]message{said("user", "hello")}, "", "", "Say SCRIPT"},
		{nil, "", "", "nothing to answer"},
	} {
		name, args, text := answer(c.messages, offered)
		if c.messages != nil && c.name == "" && c.textSays != Done {
			if _, _, untitled := answer(c.messages, nil); untitled != Untitled {
				t.Errorf("%v with no tool offered answered %q", c.messages, untitled)
			}
		}
		if name != c.name || (c.name != "" && args != c.args) || !strings.Contains(text, c.textSays) {
			t.Errorf("%v answered %q %s %q; want %q %s, saying %q", c.messages, name, args, text, c.name, c.args, c.textSays)
		}
	}
}

func TestAMessageInPartsIsReadAsItsText(t *testing.T) {
	t.Parallel()
	parts, _ := json.Marshal([]map[string]string{{"type": "text", "text": "SCRIPT"}, {"type": "text", "text": "show {}"}})
	if got := text(parts); got != "SCRIPT\nshow {}" {
		t.Errorf("read %q", got)
	}
}

// Through the handler, as a client calls it: once whole, once streamed.
func TestTheModelAnswersAsTheChatCompletionsAPI(t *testing.T) {
	t.Parallel()
	model := &Model{}
	request := func(stream bool) *http.Request {
		body, _ := json.Marshal(map[string]any{
			"model": Name, "stream": stream,
			"messages": []map[string]string{{"role": "user", "content": `SCRIPT show {"post_id":"p1"}`}},
			"tools":    []map[string]any{{"type": "function", "function": map[string]string{"name": "mattermost__show"}}},
		})
		return httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	}

	whole := httptest.NewRecorder()
	model.Handler().ServeHTTP(whole, request(false))
	var completion struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct{ Name, Arguments string } `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(whole.Body.Bytes(), &completion); err != nil || len(completion.Choices) != 1 ||
		completion.Choices[0].FinishReason != "tool_calls" || len(completion.Choices[0].Message.ToolCalls) != 1 ||
		completion.Choices[0].Message.ToolCalls[0].Function.Name != "mattermost__show" {
		t.Fatalf("answered %s (%v)", whole.Body, err)
	}

	streamed := httptest.NewRecorder()
	model.Handler().ServeHTTP(streamed, request(true))
	var events []string
	scanner := bufio.NewScanner(streamed.Body)
	for scanner.Scan() {
		if data, ok := strings.CutPrefix(scanner.Text(), "data: "); ok {
			events = append(events, data)
		}
	}
	if len(events) < 3 || events[len(events)-1] != "[DONE]" || !strings.Contains(events[0], `"mattermost__show"`) || !strings.Contains(events[1], `"tool_calls"`) {
		t.Fatalf("streamed %q", events)
	}

	requests := model.Requests()
	if len(requests) != 2 || requests[0].Answer != "mattermost__show" || len(requests[0].Tools) != 1 || !strings.Contains(requests[0].Said, "SCRIPT show") {
		t.Errorf("kept %+v", requests)
	}

	models := httptest.NewRecorder()
	model.Handler().ServeHTTP(models, httptest.NewRequest(http.MethodGet, "/models", nil))
	if !strings.Contains(models.Body.String(), Name) {
		t.Errorf("lists %s", models.Body)
	}
	bad := httptest.NewRecorder()
	model.Handler().ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader("{")))
	if bad.Code != http.StatusBadRequest {
		t.Errorf("a broken request answered %d", bad.Code)
	}
}

func TestTheScriptsEdgesAnswerInWords(t *testing.T) {
	t.Parallel()
	if _, _, text := answer([]message{said("user", "SCRIPT ")}, []string{"mattermost__show"}); !strings.Contains(text, "names no tool") {
		t.Errorf("SCRIPT and nothing: %q", text)
	}
	if _, _, text := answer([]message{said("assistant", "hello")}, []string{"mattermost__show"}); !strings.Contains(text, "Say SCRIPT") {
		t.Errorf("nothing the person said: %q", text)
	}
	if got := text(json.RawMessage(`42`)); got != "42" {
		t.Errorf("a content that is neither text nor parts: %q", got)
	}
	for name, want := range map[string]string{"show": "show", "mattermost__read_post": "read_post", "mcp_mm-mcp_get_me": "get_me", "mcp_alone": "mcp_alone"} {
		if got := Bare(name); got != want {
			t.Errorf("Bare(%q) = %q; want %q", name, got, want)
		}
	}
}

// A text answer streams as one delta and a finish.
func TestATextAnswerStreams(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]any{"stream": true, "messages": []map[string]string{{"role": "user", "content": "hello"}}})
	streamed := httptest.NewRecorder()
	(&Model{}).Handler().ServeHTTP(streamed, httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(string(body))))
	if !strings.Contains(streamed.Body.String(), Untitled) || !strings.Contains(streamed.Body.String(), `"stop"`) || !strings.HasSuffix(strings.TrimSpace(streamed.Body.String()), "[DONE]") {
		t.Errorf("streamed %s", streamed.Body)
	}
}
