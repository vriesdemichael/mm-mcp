// Package scriptedmodel is the chat model the client tests give a real MCP
// client (ADR-036): an OpenAI-compatible chat completions endpoint that
// answers from a script rather than by inference, so a client calls mm-mcp's
// tools exactly as a test says, at no cost and the same way every time.
//
// The script has two lines. A person's message that holds
//
//	SCRIPT <tool> <json arguments>
//
// is answered with a call of the offered tool <tool>, with those arguments,
// whatever the client puts before its name (Bare). A client that offers a
// large server's tools in groups, as VS Code does, offers a tool for each
// group whose description lists the tools in it: the model opens the group
// that holds the tool first, and calls the tool once it is offered, as a model
// that reads the description would. A tool's result is answered with the text
// Done. A request that offers no tool, as when a client asks
// its model to name the conversation, is answered with Untitled. Anything
// else is answered with the script itself.
//
// Every request is kept, with the tools it offered, so a test can see what a
// client puts in front of its model.
package scriptedmodel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Name is the model's id, which a client is configured to ask for.
const Name = "scripted-model"

// Server is the name a client is given for mm-mcp.
const Server = "mattermost"

// Bare is a tool's name without what a client puts before it: Goose names a
// tool after its extension, as mattermost__show, and VS Code after the
// server's own name, as mcp_mm-mcp_show.
func Bare(name string) string {
	if _, tool, ok := strings.Cut(name, "__"); ok {
		return tool
	}
	if rest, ok := strings.CutPrefix(name, "mcp_"); ok {
		if _, tool, ok := strings.Cut(rest, "_"); ok {
			return tool
		}
	}
	return name
}

// Done is the answer to a tool's result.
const Done = "Done."

// Untitled is the answer to a request that offers no tool.
const Untitled = "A scripted conversation"

// Request is one request a client sent the model.
type Request struct {
	// Tools are the names of the tools offered, as the client names them.
	Tools []string
	// Said is the last message the person sent, as text.
	Said string
	// Answer is the call the model answered with, empty for text.
	Answer string
}

// Model keeps every request a client sent it.
type Model struct {
	mu       sync.Mutex
	requests []Request
	// opened is the group the model last opened, while it waits to call the
	// tool it opened the group for.
	opened string
	ids    atomic.Int64
}

// Requests is every request so far, oldest first.
func (m *Model) Requests() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Request(nil), m.requests...)
}

type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type tool struct {
	Function offeredTool `json:"function"`
}

// offeredTool is a tool a client offers its model: its name, and what its
// description says.
type offeredTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Tools    []tool    `json:"tools"`
	Stream   bool      `json:"stream"`
}

// Handler serves the OpenAI chat completions API, with and without /v1, as
// clients differ on whether their configured address includes it.
func (m *Model) Handler() http.Handler {
	mux := http.NewServeMux()
	models := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"object": "list", "data": []map[string]any{{"id": Name, "object": "model", "owned_by": "mm-mcp"}}})
	}
	mux.HandleFunc("GET /v1/models", models)
	mux.HandleFunc("GET /models", models)
	mux.HandleFunc("POST /v1/chat/completions", m.complete)
	mux.HandleFunc("POST /chat/completions", m.complete)
	return mux
}

func (m *Model) complete(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var request chatRequest
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offered := make([]offeredTool, len(request.Tools))
	names := make([]string, len(request.Tools))
	for i, t := range request.Tools {
		offered[i], names[i] = t.Function, t.Function.Name
	}
	said := lastSaid(request.Messages)
	m.mu.Lock()
	name, arguments, text := answer(request.Messages, offered, m.opened)
	m.opened = ""
	if strings.HasPrefix(name, groupPrefix) {
		m.opened = name
	}
	m.requests = append(m.requests, Request{Tools: names, Said: said, Answer: name})
	m.mu.Unlock()
	id := fmt.Sprintf("chatcmpl-%d", m.ids.Add(1))
	if request.Stream {
		m.stream(w, id, name, arguments, text)
		return
	}
	writeJSON(w, m.completion(id, name, arguments, text))
}

// groupPrefix starts the name of a tool that opens a group of tools.
const groupPrefix = "activate_"

// answer is the script: a tool's result is answered with Done, and "SCRIPT
// <tool> <json>" in the last thing the person said with a call of the offered
// tool <tool>, or of the group that holds it. After opened, a group's result
// is answered with the call the group was opened for. name is empty when the
// answer is text.
func answer(messages []message, offered []offeredTool, opened string) (name, arguments, text string) {
	if len(messages) == 0 {
		return "", "", "There is nothing to answer."
	}
	if messages[len(messages)-1].Role == "tool" && opened == "" {
		return "", "", Done
	}
	if len(offered) == 0 {
		return "", "", Untitled
	}
	said := lastSaid(messages)
	at := strings.Index(said, "SCRIPT ")
	if at < 0 {
		return "", "", `Say SCRIPT <tool> <json arguments>, and the scripted model calls that tool.`
	}
	fields := strings.Fields(said[at+len("SCRIPT "):])
	if len(fields) == 0 {
		return "", "", "SCRIPT names no tool."
	}
	wanted := fields[0]
	arguments = "{}"
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(said[at+len("SCRIPT "):]), wanted))
	if rest != "" {
		// One JSON value: a client may add its own context after it.
		var value json.RawMessage
		if json.NewDecoder(strings.NewReader(rest)).Decode(&value) != nil {
			return "", "", fmt.Sprintf("the arguments for %s are not JSON: %s", wanted, rest)
		}
		arguments = string(value)
	}
	for _, tool := range offered {
		if Bare(tool.Name) == wanted {
			return tool.Name, arguments, ""
		}
	}
	for _, tool := range offered {
		if strings.HasPrefix(tool.Name, groupPrefix) && tool.Name != opened && holds(tool.Description, wanted) {
			return tool.Name, "{}", ""
		}
	}
	names := make([]string, len(offered))
	for i, tool := range offered {
		names[i] = tool.Name
	}
	return "", "", fmt.Sprintf("no tool offered is %s; offered: %s", wanted, strings.Join(names, ", "))
}

// lastSaid is the last message the person sent, as text.
func lastSaid(messages []message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return text(messages[i].Content)
		}
	}
	return ""
}

// text is a message's content as text, whether it is a string or parts.
func text(raw json.RawMessage) string {
	var whole string
	if json.Unmarshal(raw, &whole) == nil {
		return whole
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		said := make([]string, 0, len(parts))
		for _, part := range parts {
			said = append(said, part.Text)
		}
		return strings.Join(said, "\n")
	}
	return string(raw)
}

func (m *Model) call(name, arguments string) map[string]any {
	return map[string]any{
		"id": fmt.Sprintf("call_%d", m.ids.Add(1)), "type": "function",
		"function": map[string]any{"name": name, "arguments": arguments},
	}
}

func (m *Model) completion(id, name, arguments, text string) map[string]any {
	answer := map[string]any{"role": "assistant", "content": text}
	finish := "stop"
	if name != "" {
		answer["content"] = nil
		answer["tool_calls"] = []map[string]any{m.call(name, arguments)}
		finish = "tool_calls"
	}
	return map[string]any{
		"id": id, "object": "chat.completion", "created": time.Now().Unix(), "model": Name,
		"choices": []map[string]any{{"index": 0, "message": answer, "finish_reason": finish}},
		"usage":   usage,
	}
}

var usage = map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}

// stream answers as server-sent events, in one delta and a finish.
func (m *Model) stream(w http.ResponseWriter, id, name, arguments, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	send := func(chunk map[string]any) {
		chunk["id"], chunk["object"], chunk["created"], chunk["model"] = id, "chat.completion.chunk", time.Now().Unix(), Name
		data, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	delta := func(delta map[string]any, finish any) map[string]any {
		return map[string]any{"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}}
	}
	if name != "" {
		call := m.call(name, arguments)
		call["index"] = 0
		send(delta(map[string]any{"role": "assistant", "content": nil, "tool_calls": []map[string]any{call}}, nil))
		send(delta(map[string]any{}, "tool_calls"))
	} else {
		send(delta(map[string]any{"role": "assistant", "content": text}, nil))
		send(delta(map[string]any{}, "stop"))
	}
	send(map[string]any{"choices": []map[string]any{}, "usage": usage})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// holds reports whether a group's description lists the tool wanted, as
// VS Code's does: "Contains the tools: a, b, c".
func holds(description, wanted string) bool {
	_, listed, ok := strings.Cut(description, "Contains the tools: ")
	if !ok {
		return false
	}
	for _, part := range strings.Split(listed, ",") {
		if fields := strings.Fields(part); len(fields) > 0 && Bare(strings.TrimRight(fields[0], ".")) == wanted {
			return true
		}
	}
	return false
}
