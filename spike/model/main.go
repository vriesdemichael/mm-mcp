//go:build spike

// Command model is the spike's scripted model (#37): an OpenAI-compatible chat
// completions endpoint on the loopback address that answers without inference.
// A user message "SPIKE <tool> <json arguments>" is answered with a call of
// the offered tool whose name ends in <tool>; a tool result is answered with a
// short text. Every request is appended to the log, and every marker string the
// spike planted (SPIKE-META- in _meta only, SPIKE-SC- in structuredContent,
// SPIKE-CTX- from the view's context update, SPIKE-TOKEN-) that reaches
// the model is reported, which shows what a client passes to its model.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var markers = regexp.MustCompile(`SPIKE-(META|SC|CTX|TOKEN|RES)-[0-9a-f]{8}`)

type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type tool struct {
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Tools    []tool    `json:"tools"`
	Stream   bool      `json:"stream"`
}

var calls atomic.Int64

func main() {
	address := flag.String("listen", "127.0.0.1:18080", "loopback address to listen on")
	logPath := flag.String("log", "spike-model.jsonl", "file every request is appended to")
	flag.Parse()
	file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	logger := log.New(io.MultiWriter(os.Stderr, file), "spike model: ", log.LstdFlags|log.Lmicroseconds)
	mux := http.NewServeMux()
	models := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"object": "list", "data": []map[string]any{{"id": "spike-model", "object": "model", "owned_by": "spike"}}})
	}
	mux.HandleFunc("/v1/models", models)
	mux.HandleFunc("/models", models)
	completions := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		logger.Printf("request %s", body)
		for _, seen := range markers.FindAllString(string(body), -1) {
			logger.Printf("MARKER REACHED THE MODEL: %s", seen)
		}
		var request chatRequest
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name, arguments, text := decide(request)
		logger.Printf("answering: tool=%q arguments=%s text=%q", name, arguments, text)
		if request.Stream {
			stream(w, request.Model, name, arguments, text)
			return
		}
		writeJSON(w, complete(request.Model, name, arguments, text))
	}
	mux.HandleFunc("/v1/chat/completions", completions)
	mux.HandleFunc("/chat/completions", completions)
	logger.Printf("listening on %s", *address)
	server := &http.Server{Addr: *address, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(server.ListenAndServe())
}

// decide is the script: a tool result gets a text; "SPIKE <tool> <json>" gets
// that tool's call; anything else gets the instructions.
func decide(request chatRequest) (name, arguments, text string) {
	if len(request.Messages) == 0 {
		return "", "", "spike model: no messages"
	}
	last := request.Messages[len(request.Messages)-1]
	if last.Role == "tool" {
		return "", "", "Spike: the tool answered."
	}
	for i := len(request.Messages) - 1; i >= 0; i-- {
		if request.Messages[i].Role != "user" {
			continue
		}
		said := contentText(request.Messages[i].Content)
		index := strings.Index(said, "SPIKE ")
		if index < 0 {
			break
		}
		fields := strings.SplitN(strings.TrimSpace(said[index+len("SPIKE "):]), " ", 2)
		args := "{}"
		if len(fields) == 2 {
			// One JSON value: a client may append its own context after it.
			var value json.RawMessage
			if json.NewDecoder(strings.NewReader(fields[1])).Decode(&value) == nil {
				args = string(value)
			}
		}
		fields[0] = strings.Fields(fields[0])[0]
		for _, offered := range request.Tools {
			if strings.HasSuffix(offered.Function.Name, fields[0]) {
				return offered.Function.Name, args, ""
			}
		}
		return "", "", fmt.Sprintf("spike model: no offered tool ends in %q", fields[0])
	}
	return "", "", "spike model: send SPIKE <tool> <json arguments>"
}

func contentText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var all []string
		for _, part := range parts {
			all = append(all, part.Text)
		}
		return strings.Join(all, "\n")
	}
	return string(raw)
}

func complete(model, name, arguments, text string) map[string]any {
	messageBody := map[string]any{"role": "assistant", "content": text}
	finish := "stop"
	if name != "" {
		messageBody["content"] = nil
		messageBody["tool_calls"] = []map[string]any{toolCall(name, arguments)}
		finish = "tool_calls"
	}
	return map[string]any{
		"id": fmt.Sprintf("chatcmpl-spike-%d", calls.Add(1)), "object": "chat.completion", "created": time.Now().Unix(), "model": model,
		"choices": []map[string]any{{"index": 0, "message": messageBody, "finish_reason": finish}},
		"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	}
}

func toolCall(name, arguments string) map[string]any {
	return map[string]any{"id": fmt.Sprintf("call_spike_%d", calls.Add(1)), "type": "function", "function": map[string]any{"name": name, "arguments": arguments}}
}

func stream(w http.ResponseWriter, model, name, arguments, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	id := fmt.Sprintf("chatcmpl-spike-%d", calls.Add(1))
	send := func(delta map[string]any, finish any) {
		chunk := map[string]any{"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
			"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	if name != "" {
		call := toolCall(name, arguments)
		call["index"] = 0
		send(map[string]any{"role": "assistant", "content": nil, "tool_calls": []map[string]any{call}}, nil)
		send(map[string]any{}, "tool_calls")
	} else {
		send(map[string]any{"role": "assistant", "content": text}, nil)
		send(map[string]any{}, "stop")
	}
	usage, _ := json.Marshal(map[string]any{"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
		"choices": []map[string]any{}, "usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}})
	fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", usage)
	if flusher != nil {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
