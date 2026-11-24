// display-set (box-desktop.md §4, Control Panel's Display): how the box's own
// screen shows, kept for the box rather than for one admin, since the monitor
// is the family's: /etc/zoomriya/display.conf, sway's output lines, which
// screen-session adds to sway's settings each time the screen starts. Box
// Desktop tries a change live first (swaymsg) and asks for it to be saved only
// once the admin keeps it; "forget" goes back to the monitor's own best.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const displayFile = "etc/zoomriya/display.conf"

var (
	outputName  = regexp.MustCompile(`^[A-Za-z0-9-]{1,32}$`)
	displaySize = regexp.MustCompile(`^[1-9][0-9]{2,4}x[1-9][0-9]{2,4}$`)
	transforms  = map[string]bool{"normal": true, "90": true, "180": true, "270": true}
	scales      = map[float64]bool{1: true, 1.25: true, 1.5: true, 1.75: true, 2: true}
)

// displayLines are display.conf's lines by output, as written.
func (a *Agent) displayLines() map[string]string {
	lines := map[string]string{}
	for _, line := range strings.Split(a.readTrimmed(displayFile), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "output" {
			lines[fields[1]] = line
		}
	}
	return lines
}

func (a *Agent) displaySet(req Request) (Response, error) {
	if req.Output == nil || !outputName.MatchString(*req.Output) {
		return nil, refuse("Which screen? None was given.")
	}
	lines := a.displayLines()
	if req.Action != nil && *req.Action == "forget" {
		delete(lines, *req.Output)
	} else {
		if req.Size == nil || !displaySize.MatchString(*req.Size) {
			return nil, refuse("Which resolution? Choose one from the list.")
		}
		if req.Refresh == nil || *req.Refresh < 1000 || *req.Refresh > 1000000 {
			return nil, refuse("Which refresh rate? Choose one from the list.")
		}
		if req.Scale == nil || !scales[*req.Scale] {
			return nil, refuse("Which scale? Choose one from the list.")
		}
		if req.Transform == nil || !transforms[*req.Transform] {
			return nil, refuse("Which way round? Choose one from the list.")
		}
		lines[*req.Output] = fmt.Sprintf("output %s mode %s@%.3fHz scale %g transform %s",
			*req.Output, *req.Size, float64(*req.Refresh)/1000, *req.Scale, *req.Transform)
	}

	names := make([]string, 0, len(lines))
	for name := range lines {
		names = append(names, name)
	}
	sort.Strings(names)
	text := "# The box's screen, as an admin kept it in Control Panel (zoomriya-agent's\n" +
		"# display-set). screen-session adds these lines to sway's settings.\n"
	for _, name := range names {
		text += lines[name] + "\n"
	}
	file := a.path(displayFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(file+".new", []byte(text), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(file+".new", file); err != nil {
		return nil, err
	}
	return Response{"output": *req.Output, "saved": lines[*req.Output]}, nil
}
