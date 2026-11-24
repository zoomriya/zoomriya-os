// The OS's own Myira (box-desktop.md §4, Myira): Box Desktop's help when
// the Myira app isn't on for this admin. It knows only the box: a how-to is
// one of Support Center's Help topics, word for word, with a button to its
// tool and section; the box's state comes from the OS's tools, and only then
// does the model put it in a few plain words. Any other question gets one
// fixed reply. It keeps no chats: the panel's chat is gone when it's closed.
//
// Written for this repository: it holds no other Myira's code or words. It
// answers in the assistant contract's shape (docs/assistant-contract.md), so
// the panel shows either Myira the same way. Its fixed replies are sent as a
// "say" code and worded by the page, in the admin's language.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type osMyira struct{ s *Server }

func (s *Server) osMyira() osMyira { return osMyira{s} }

// The page's words for each "say" code (Box Desktop's po/).
const (
	sayNotBox      = "not_box"     // "For now I can help with the box. With the ZOOMRIYA apps installed, I can answer much more."
	sayNoModel     = "no_model"    // "Myira needs her model to answer (3.2 GB)."
	sayUnavailable = "unavailable" // "Myira can't answer right now. Try again in a moment."
	sayDeclined    = "declined"    // "Nothing was changed."
	// "That asks for your password again, so it's done in {tool} itself." (with open)
	sayPassword = "needs_password"
	// "If someone is hurt or in danger, call {number} now." (with number), or
	// without one: "…call your local emergency number now."
	sayEmergency = "emergency"
)

// passwordPlaces: what asks for the password again (§7), never done by
// Myira: she opens the place where it's done.
var passwordPlaces = []struct{ What, Tool, Section string }{
	{"Erase everything, reset the system, or restore the OS to an earlier restore point", "control", "restore"},
	{"Restart or shut down the box", "control", "restarts"},
	{"Turn on advanced access (SSH) or add an SSH key", "control", "access"},
	{"Make someone an admin or not, or turn an account off or on", "control", "people"},
}

// ---- someone hurt or in danger (BD-18, decided 14 November 2026: a fixed line
// with the emergency number of where the family lives, before anything else) ----

var emergencyWords = regexp.MustCompile(`(?i)\b(?:emergency|hurt|injured|bleeding|can'?t breathe|not breathing|chest pain|` +
	`heart attack|stroke|overdos\w*|poison\w*|suicid\w*|kill (?:my|him|her)self|want to die|unconscious|` +
	`(?:house|kitchen) (?:is )?on fire|there'?s a fire|break-?in|broke in|intruder|in danger|` +
	`urgence|bless(?:é|e)e?s?|saigne\w*|respire (?:pas|plus)|douleur (?:à|a) la poitrine|crise cardiaque|` +
	`surdose|empoisonn\w*|suicid\w*|me tuer|envie de mourir|inconscient\w*|au feu|cambriol\w*|en danger)`)

// emergencyNumbers: the number for an ambulance, the police and the fire
// service, by country, where one number answers for all; any other country
// gets "your local emergency number", never a guess.
var emergencyNumbers = map[string]string{
	"CA": "911", "US": "911", "MX": "911",
	"GB": "999", "AU": "000", "NZ": "111", "IN": "112",
	"AT": "112", "BE": "112", "BG": "112", "HR": "112", "CY": "112", "CZ": "112", "DK": "112", "EE": "112",
	"FI": "112", "FR": "112", "DE": "112", "GR": "112", "HU": "112", "IE": "112", "IT": "112", "LV": "112",
	"LT": "112", "LU": "112", "MT": "112", "NL": "112", "PL": "112", "PT": "112", "RO": "112", "SK": "112",
	"SI": "112", "ES": "112", "SE": "112", "NO": "112", "CH": "112", "IS": "112",
}

// emergency: the fixed reply when the question says someone is hurt or in
// danger, or nil.
func (m osMyira) emergency(question, by string) map[string]any {
	if !emergencyWords.MatchString(question) {
		return nil
	}
	done := map[string]any{"type": "done", "answer": "", "choices": []any{}, "conversation_id": nil, "question_id": nil,
		"mode": "box", "fixed": true, "say": sayEmergency}
	if answer, p := m.s.Agent.Ask(by, "setup-status", nil); p == nil {
		region, _ := answer["region"].(string)
		country, _, _ := strings.Cut(strings.ToUpper(region), "-")
		if number := emergencyNumbers[country]; number != "" {
			done["number"] = number
		}
	}
	return done
}

var languageNames = map[string]string{"en": "English", "fr": "French"}

// modelRequest asks the box's model service as the admin, renewing their
// token once if it ran out.
func (m osMyira) modelRequest(ctx context.Context, login *Login, path string, body any) (*http.Response, error) {
	raw, _ := json.Marshal(body)
	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.s.Cfg.ModelAPI+path, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+login.Token)
		request.Header.Set("Content-Type", "application/json")
		response, err := myiraClient.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode == http.StatusUnauthorized && attempt == 0 && m.s.ID.Renew(login) {
			response.Body.Close()
			continue
		}
		return response, nil
	}
}

// ---- picking: a Help topic, a tool, or neither --------------------------------------

func pickPrompt(help []HelpTopic) string {
	var b strings.Builder
	b.WriteString("You choose how Box Desktop's help answers an admin's question about their ZOOMRIYA Box, " +
		"a home server. Reply with JSON only: {\"pick\": ..., \"arguments\": {...}}.\n\n" +
		"Pick a help topic (\"help:N\") when the question asks how to do something or what to do about a problem it covers. " +
		"Pick a tool (\"box.NAME\") when the question asks about the box's state now, or asks to change something a tool changes; " +
		"fill in its arguments. Pick \"open:N\" when the question asks to do something listed under \"Asks for the password\". " +
		"Pick \"other\" when the question isn't about the box (general knowledge, the family's photos, " +
		"files or chats, anything else), or nothing here fits.\n\nHelp topics:\n")
	for i, topic := range help {
		fmt.Fprintf(&b, "- help:%d: %s. %s\n", i, topic.Title, topic.Text)
	}
	b.WriteString("\nAsks for the password (never a tool):\n")
	for i, place := range passwordPlaces {
		fmt.Fprintf(&b, "- open:%d: %s\n", i, place.What)
	}
	b.WriteString("\nTools:\n")
	for _, tool := range boxTools {
		kind := "reads"
		if tool.Writes {
			kind = "changes something"
		}
		fmt.Fprintf(&b, "- box.%s (%s): %s", tool.Name, kind, tool.Description)
		if props, _ := tool.InputSchema["properties"].(map[string]any); len(props) > 0 {
			raw, _ := json.Marshal(props)
			fmt.Fprintf(&b, " Arguments: %s", raw)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func pickSchema(help []HelpTopic) map[string]any {
	choices := []string{}
	for i := range help {
		choices = append(choices, "help:"+strconv.Itoa(i))
	}
	for i := range passwordPlaces {
		choices = append(choices, "open:"+strconv.Itoa(i))
	}
	for _, tool := range boxTools {
		choices = append(choices, toolApp+"."+tool.Name)
	}
	choices = append(choices, "other")
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pick":      map[string]any{"type": "string", "enum": choices},
			"arguments": map[string]any{"type": "object"},
		},
		"required": []string{"pick"},
	}
}

var jsonObject = regexp.MustCompile(`(?s)\{.*\}`)

// pick asks the model once. found: false when it couldn't be asked.
func (m osMyira) pick(ctx context.Context, login *Login, question string, help []HelpTopic) (pick string, args map[string]any, err error) {
	response, err := m.modelRequest(ctx, login, "/complete", map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": pickPrompt(help)},
			{"role": "user", "content": question},
		},
		"max_tokens": 200, "temperature": 0, "json_schema": pickSchema(help),
	})
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("the model service: %s", response.Status)
	}
	var answer struct {
		Reply string `json:"reply"`
	}
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return "", nil, err
	}
	var chosen struct {
		Pick      string         `json:"pick"`
		Arguments map[string]any `json:"arguments"`
	}
	if found := jsonObject.FindString(answer.Reply); found == "" || json.Unmarshal([]byte(found), &chosen) != nil {
		return "other", nil, nil
	}
	return chosen.Pick, chosen.Arguments, nil
}

// Ask answers a question: a Help topic word for word, a tool for the panel
// to run, or the fixed reply.
func (m osMyira) Ask(w http.ResponseWriter, r *http.Request, login *Login, b askBody) {
	// Someone hurt or in danger: before anything else, with or without a model.
	if done := m.emergency(b.Question, login.Member.Username); done != nil {
		out := stream(w)
		out.send(route())
		out.send(done)
		return
	}
	if !m.s.modelLoaded(r.Context()) {
		out := stream(w)
		out.send(route())
		out.send(map[string]any{"type": "error", "say": sayNoModel})
		return
	}
	picked, args, err := m.pick(r.Context(), login, b.Question, b.Help)
	out := stream(w)
	out.send(route())
	if err != nil {
		out.send(map[string]any{"type": "error", "say": sayUnavailable})
		return
	}
	done := map[string]any{"type": "done", "answer": "", "choices": []any{}, "conversation_id": nil, "question_id": nil, "mode": "box"}
	switch {
	case strings.HasPrefix(picked, "help:"):
		i, err := strconv.Atoi(strings.TrimPrefix(picked, "help:"))
		if err != nil || i < 0 || i >= len(b.Help) {
			done["say"], done["fixed"] = sayNotBox, true
			break
		}
		topic := b.Help[i]
		// Help's own words, as Support Center shows them, and its button.
		out.send(map[string]any{"type": "text", "text": topic.Text})
		done["answer"], done["fixed"] = topic.Text, true
		done["source"] = map[string]any{"name": topic.Title}
		done["open"] = map[string]any{"tool": topic.Tool, "section": topic.Section}
	case strings.HasPrefix(picked, "open:"):
		i, err := strconv.Atoi(strings.TrimPrefix(picked, "open:"))
		if err != nil || i < 0 || i >= len(passwordPlaces) {
			done["say"], done["fixed"] = sayNotBox, true
			break
		}
		done["say"], done["fixed"] = sayPassword, true
		done["open"] = map[string]any{"tool": passwordPlaces[i].Tool, "section": passwordPlaces[i].Section}
	case strings.HasPrefix(picked, toolApp+"."):
		tool := findTool(toolApp, strings.TrimPrefix(picked, toolApp+"."))
		if tool == nil {
			done["say"], done["fixed"] = sayNotBox, true
			break
		}
		known := map[string]any{}
		if props, _ := tool.InputSchema["properties"].(map[string]any); props != nil {
			for k, v := range args {
				if _, ok := props[k]; ok {
					known[k] = v
				}
			}
		}
		event := map[string]any{"app": toolApp, "name": tool.Name, "title": tool.Title, "arguments": known, "writes": tool.Writes}
		flat := map[string]any{"type": "tool"}
		for k, v := range event {
			flat[k] = v
		}
		out.send(flat)
		done["tool"] = event
	default:
		done["say"], done["fixed"] = sayNotBox, true
	}
	out.send(done)
}

func route() map[string]any {
	return map[string]any{"type": "route", "conversation_id": nil, "mode": "box", "fixed": false}
}

// answerPrompt: the model's few words from what a tool said, and nothing else.
func answerPrompt(step Step, lang string) string {
	result := strings.ReplaceAll(step.Result, `"""`, `'''`)
	var what string
	if step.IsError {
		what = fmt.Sprintf("Box Desktop's tool %q couldn't do it. What it said:\n\"\"\"\n%s\n\"\"\"\nSay so simply.", step.Title, result)
	} else {
		what = fmt.Sprintf("Box Desktop's tool %q answered:\n\"\"\"\n%s\n\"\"\"", step.Title, result)
	}
	return "You are Myira, the help in Box Desktop on a ZOOMRIYA Box, a family's home server, answering one of its admins. " +
		"Answer their question from the tool's answer below and nothing else, in at most three short sentences of plain words. " +
		"Use the figures exactly as they are written, never work out new ones. Never guess what the answer doesn't say, never give a link, never suggest going online. " +
		"Reply in " + languageNames[lang] + ".\n\n" + what
}

// FromTool answers from what the OS's tool said, as the model writes it.
func (m osMyira) FromTool(w http.ResponseWriter, r *http.Request, login *Login, b toolBody, step Step) {
	done := map[string]any{"type": "done", "answer": "", "choices": []any{}, "conversation_id": nil, "question_id": nil, "mode": "box"}
	if step.Declined {
		out := stream(w)
		out.send(route())
		done["say"], done["fixed"] = sayDeclined, true
		out.send(done)
		return
	}
	response, err := m.modelRequest(r.Context(), login, "/complete/stream", map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": answerPrompt(step, b.Lang)},
			{"role": "user", "content": strings.TrimSpace(b.Question)},
		},
		"max_tokens": 200, "temperature": 0.2,
	})
	out := stream(w)
	out.send(route())
	if err != nil {
		out.send(map[string]any{"type": "error", "say": sayUnavailable})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		say := sayUnavailable
		if response.StatusCode == http.StatusServiceUnavailable && !m.s.modelLoaded(r.Context()) {
			say = sayNoModel
		}
		out.send(map[string]any{"type": "error", "say": say})
		return
	}
	var answer strings.Builder
	reader := bufio.NewReaderSize(response.Body, 1<<16)
	for {
		line, err := reader.ReadBytes('\n')
		var piece struct {
			Text  string `json:"text"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if len(bytes.TrimSpace(line)) > 0 && json.Unmarshal(line, &piece) == nil {
			switch {
			case piece.Error != "":
				out.send(map[string]any{"type": "error", "say": sayUnavailable})
				return
			case piece.Text != "":
				answer.WriteString(piece.Text)
				out.send(map[string]any{"type": "text", "text": piece.Text})
			case piece.Done:
				done["answer"] = strings.TrimSpace(answer.String())
				out.send(done)
				return
			}
		}
		if err != nil {
			out.send(map[string]any{"type": "error", "say": sayUnavailable})
			return
		}
	}
}
