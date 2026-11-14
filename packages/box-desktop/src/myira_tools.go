// The OS's tools for Myira (box-desktop.md §4, Myira): what Myira may look
// at and change on the box, offered with each question from Box Desktop's
// panel, in the apps' tool shape (the Myira app's family_tools.ToolOffer:
// app, name, title, description, input schema, writes). Whichever Myira
// answers only picks one; box-desktopd runs it here, as the signed-in admin,
// after the admin confirmed a change on its card.
//
// Never a tool: anything that asks for the password again (§7: Erase
// everything, Reset, Restore, Restart, Shut down, advanced access, making or
// removing an admin, turning an account off). Myira opens that tool's section
// instead. These tools are offered nowhere but this panel.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// ToolOffer is one tool, as the Myira app reads it (contract 11).
type ToolOffer struct {
	App         string         `json:"app"`
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Writes      bool           `json:"writes"`
}

// boxTool: a tool and how it is run.
type boxTool struct {
	ToolOffer
	run func(s *Server, by string, args map[string]any) (any, error)
}

// toolApp: the OS's tools' app name in every offer and pick.
const toolApp = "box"

var noArgs = map[string]any{"type": "object", "properties": map[string]any{}}

var hourPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// agentTool: a tool that is one agent request, with fixed fields.
func agentTool(op string, fields map[string]any) func(s *Server, by string, _ map[string]any) (any, error) {
	return func(s *Server, by string, _ map[string]any) (any, error) {
		return s.askAgent(by, op, fields)
	}
}

func (s *Server) askAgent(by, op string, fields map[string]any) (any, error) {
	raw := map[string]json.RawMessage{}
	for k, v := range fields {
		raw[k], _ = json.Marshal(v)
	}
	if rule, ok := agentRules[op]; !ok || rule.needsPassword(raw) || rule.Screen || rule.SignedOut {
		return nil, fmt.Errorf("not a tool: %s", op)
	}
	answer, p := s.Agent.Ask(by, op, raw)
	if p != nil {
		if p.Error != "" {
			return nil, fmt.Errorf("%s", p.Error)
		}
		return nil, fmt.Errorf("the box didn't answer (%s)", p.Code)
	}
	return answer, nil
}

// boxTools: every tool, in the order the model reads them.
var boxTools = []boxTool{
	{ToolOffer{toolApp, "box_status", "The box", "The box's OS version and how long it gets security fixes.", noArgs, false},
		agentTool("status", nil)},
	{ToolOffer{toolApp, "storage", "Storage", "How full the box's storage is: the family's library and the system, in bytes.", noArgs, false},
		agentTool("storage", nil)},
	{ToolOffer{toolApp, "drive_health", "Drive health", "Each drive's health, temperature and age, and whether one should be replaced.", noArgs, false},
		agentTool("disk-health", nil)},
	{ToolOffer{toolApp, "backups", "Backups", "Whether backups are set up, where to, when the last one ran, and the nightly backup.", noArgs, false},
		agentTool("backups", nil)},
	{ToolOffer{toolApp, "security", "Security and updates", "Security fixes (automatic, last check, last installed, waiting), the firewall, the certificate and the box's other protections.", noArgs, false},
		agentTool("security", nil)},
	{ToolOffer{toolApp, "restore_points", "Restore points", "The OS's restore points, newest first.", noArgs, false},
		agentTool("restore-point-list", nil)},
	{ToolOffer{toolApp, "network", "Network", "The box's network: its address, cable or Wi-Fi, the router and DNS.", noArgs, false},
		agentTool("network", nil)},
	{ToolOffer{toolApp, "resources", "Resource Monitor", "What is using the box now: processor, memory, disk and each app.", noArgs, false},
		agentTool("resources", nil)},
	{ToolOffer{toolApp, "maintenance_hour", "Maintenance hour", "The hour the box restarts by itself when an update or a restore waits for it.", noArgs, false},
		agentTool("maintenance-hour", nil)},
	{ToolOffer{toolApp, "sound", "Sound", "The box's speakers and microphone, and their volume.", noArgs, false},
		func(s *Server, _ string, _ map[string]any) (any, error) {
			return map[string]any{"output": s.Hardware.Devices(true), "input": s.Hardware.Devices(false)}, nil
		}},

	{ToolOffer{toolApp, "set_maintenance_hour", "Change the maintenance hour", "Move the hour the box restarts by itself for updates.",
		map[string]any{"type": "object", "required": []string{"hour"}, "properties": map[string]any{
			"hour": map[string]any{"type": "string", "description": "The time of day, HH:MM, 24-hour (03:30)"}}}, true},
		func(s *Server, by string, args map[string]any) (any, error) {
			hour, _ := args["hour"].(string)
			if !hourPattern.MatchString(hour) {
				return nil, fmt.Errorf("that isn't a time of day: %q", hour)
			}
			return s.askAgent(by, "maintenance-hour", map[string]any{"hour": hour})
		}},
	{ToolOffer{toolApp, "backup_now", "Back up now", "Start a backup now, when backups are set up.", noArgs, true},
		agentTool("backups", map[string]any{"action": "now"})},
	{ToolOffer{toolApp, "check_updates_now", "Check for security fixes now", "Check for and install the OS's security fixes now.", noArgs, true},
		agentTool("security-check-now", nil)},
	{ToolOffer{toolApp, "set_volume", "Change the volume", "Set the box's speaker volume.",
		map[string]any{"type": "object", "required": []string{"percent"}, "properties": map[string]any{
			"percent": map[string]any{"type": "integer", "minimum": 0, "maximum": 100, "description": "The volume, 0 to 100"}}}, true},
		func(s *Server, _ string, args map[string]any) (any, error) {
			percent, ok := args["percent"].(float64)
			if !ok || percent < 0 || percent > 100 {
				return nil, fmt.Errorf("the volume is 0 to 100")
			}
			s.Hardware.SetVolume(true, percent/100)
			return map[string]any{"volume_percent": int(percent)}, nil
		}},
}

// Offers: the tools as the panel offers them with each question.
func Offers() []ToolOffer {
	offers := make([]ToolOffer, len(boxTools))
	for i, t := range boxTools {
		offers[i] = t.ToolOffer
	}
	return offers
}

func findTool(app, name string) *boxTool {
	if app != toolApp {
		return nil
	}
	for i := range boxTools {
		if boxTools[i].Name == name {
			return &boxTools[i]
		}
	}
	return nil
}

// mostResult: of a tool's answer, what Myira reads (the app's MOST_RESULT).
const mostResult = 12000

// RunTool runs one of the OS's tools as the admin: what came of it, as text,
// and whether it couldn't.
func (s *Server) RunTool(by, app, name, lang string, args map[string]any) (string, bool) {
	tool := findTool(app, name)
	if tool == nil {
		return "There is no such tool on the box.", true
	}
	if args == nil {
		args = map[string]any{}
	}
	answer, err := tool.run(s, by, args)
	if err != nil {
		return err.Error(), true
	}
	raw, _ := json.Marshal(readable(answer, "", lang))
	if len(raw) > mostResult {
		raw = raw[:mostResult]
	}
	return string(raw), false
}

// sizeKey: a field that holds a number of bytes.
var sizeKey = regexp.MustCompile(`(?i)(^|_)(size|used|free|bytes|total|available|capacity)($|_)`)

// readable: a tool's answer with every size in bytes as the box's screens say
// it ("3.1 TB"), and the share used beside a size and what's used of it, so
// the small model on the box repeats figures instead of working them out.
func readable(v any, key, lang string) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x)+1)
		for k, inner := range x {
			out[k] = readable(inner, k, lang)
		}
		size, sized := number(x["size"])
		used, isUsed := number(x["used"])
		if sized && isUsed && size > 0 {
			out["used_percent"] = int(math.Round(used / size * 100))
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, inner := range x {
			out[i] = readable(inner, key, lang)
		}
		return out
	default:
		if n, ok := number(v); ok && sizeKey.MatchString(key) && !strings.Contains(strings.ToLower(key), "percent") && n >= 1000 {
			return humanBytes(n, lang)
		}
		return v
	}
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// humanBytes: decimal units, as drives are sold, in the admin's language.
func humanBytes(n float64, lang string) string {
	units := []string{"bytes", "kB", "MB", "GB", "TB", "PB"}
	if lang == "fr" {
		units = []string{"octets", "ko", "Mo", "Go", "To", "Po"}
	}
	i := 0
	for n >= 1000 && i < len(units)-1 {
		n /= 1000
		i++
	}
	text := fmt.Sprintf("%.1f %s", n, units[i])
	if n >= 100 || i == 0 {
		text = fmt.Sprintf("%.0f %s", n, units[i])
	}
	text = strings.Replace(text, ".0 ", " ", 1)
	if lang == "fr" {
		text = strings.Replace(text, ".", ",", 1)
	}
	return text
}
