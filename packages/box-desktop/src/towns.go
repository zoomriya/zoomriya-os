// The family's town and the box's weather (weather.md): the towns people
// choose from in the first setup and Control Panel, searched within the
// chosen country, bigger towns first; and the weather zoomriya-weather keeps,
// for the desktop's widget. Box Desktop only reads both.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	townsFile   = "usr/share/zoomriya/towns.tsv.gz"
	weatherFile = "var/lib/zoomriya-weather/weather.json"
	// How many towns a search lists.
	townsShown = 20
)

// TownChoice is one town in the list.
type TownChoice struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Admin1     string `json:"admin1"`
	Population int64  `json:"population"`
	folded     string
}

type townCache struct {
	mu        sync.Mutex
	byCountry map[string][]TownChoice
}

var towns townCache

// unaccent: the Latin letters with accents in towns' names, without them.
var unaccent = func() *strings.Replacer {
	pairs := []string{}
	for plain, accented := range map[string]string{
		"a": "àáâãäåāăą", "c": "çćĉċč", "d": "ďđ", "e": "èéêëēĕėęě", "g": "ĝğġģ", "h": "ĥħ",
		"i": "ìíîïĩīĭįı", "j": "ĵ", "k": "ķ", "l": "ĺļľŀł", "n": "ñńņňŉ", "o": "òóôõöøōŏő",
		"r": "ŕŗř", "s": "śŝşšș", "t": "ţťŧț", "u": "ùúûüũūŭůűų", "w": "ŵ", "y": "ýÿŷ", "z": "źżž",
		"ae": "æ", "oe": "œ", "ss": "ß",
	} {
		for _, r := range accented {
			pairs = append(pairs, string(r), plain)
		}
	}
	return strings.NewReplacer(pairs...)
}()

// fold: lower case without accents, for matching what people type.
func fold(s string) string { return unaccent.Replace(strings.ToLower(s)) }

// countryTowns: a country's towns, bigger first (as the file has them), read
// once.
func (p *Places) countryTowns(region string) []TownChoice {
	towns.mu.Lock()
	defer towns.mu.Unlock()
	if found, ok := towns.byCountry[region]; ok {
		return found
	}
	list := []TownChoice{}
	f, err := os.Open(p.path(townsFile))
	if err != nil {
		return list
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return list
	}
	lines := bufio.NewScanner(z)
	lines.Buffer(make([]byte, 64<<10), 1<<20)
	tab := "\t" + region + "\t"
	for lines.Scan() {
		line := lines.Text()
		if !strings.Contains(line, tab) {
			continue
		}
		// id, name, ascii, country, admin1, lat, lon, population, time zone
		f := strings.Split(line, "\t")
		if len(f) < 9 || f[3] != region {
			continue
		}
		id, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			continue
		}
		pop, _ := strconv.ParseInt(f[7], 10, 64)
		list = append(list, TownChoice{ID: id, Name: f[1], Admin1: f[4], Population: pop,
			folded: fold(f[1]) + "\x00" + fold(f[2])})
	}
	if towns.byCountry == nil {
		towns.byCountry = map[string][]TownChoice{}
	}
	towns.byCountry[region] = list
	return list
}

// Towns: the region's towns whose name, or a word of it, starts with what
// was typed; the biggest first. Nothing typed: the biggest.
func (p *Places) Towns(region, typed string) []TownChoice {
	q := strings.TrimSpace(fold(typed))
	found := []TownChoice{}
	for _, t := range p.countryTowns(region) {
		if q == "" || matchesStart(t.folded, q) {
			found = append(found, t)
			if len(found) == townsShown {
				break
			}
		}
	}
	return found
}

func matchesStart(folded, q string) bool {
	if strings.HasPrefix(folded, q) {
		return true
	}
	for i := 0; i < len(folded); i++ {
		switch folded[i] {
		case ' ', '-', '\'', '\x00':
			if strings.HasPrefix(folded[i+1:], q) {
				return true
			}
		}
	}
	return false
}

func (s *Server) townsIn(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	region := r.URL.Query().Get("region")
	if len(region) != 2 || strings.ToUpper(region) != region {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	writeJSON(w, http.StatusOK, s.Places.Towns(region, r.URL.Query().Get("q")))
}

// weather: what zoomriya-weather kept, for the widget; {"state": "none"}
// before a town is chosen.
func (s *Server) weather(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	data, err := os.ReadFile(s.Places.path(weatherFile))
	var kept map[string]any
	if err != nil || json.Unmarshal(data, &kept) != nil {
		writeJSON(w, http.StatusOK, map[string]any{"state": "none"})
		return
	}
	writeJSON(w, http.StatusOK, kept)
}
