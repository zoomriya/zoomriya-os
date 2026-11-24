package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Alerts outside Canada and the US (weather.md W-4), for the family's one
// town only: never another country's feed, never another town's alerts.
//
//   - Norway: MET Norway's MetAlerts, asked for the town's point (CC BY 4.0).
//   - Europe: EUMETNET – MeteoAlarm, the town's country's Atom feed only
//     (CC BY 4.0, credited "EUMETNET – MeteoAlarm" with a link). A warning
//     whose area code is one of the town's (worked out when the OS is built,
//     towns.tsv.gz) is kept; in the countries whose warnings carry no code,
//     each new warning's CAP is read once, cached, and kept only when one of
//     its areas' polygons holds the town.

// --- MET Norway's MetAlerts ---------------------------------------------------------

type MetAlerts struct {
	F    *Fetcher
	Base string // https://api.met.no
}

var metAlertsCredit = Words{"Weather alerts from MET Norway, CC BY 4.0.", "Alertes météo de MET Norway, CC BY 4.0."}

func (m *MetAlerts) Official(Town) (Words, Words) {
	return same("https://www.yr.no/en"), Words{"MET Norway's warnings on Yr", "les avertissements de MET Norway sur Yr"}
}

func (m *MetAlerts) Credit() Words { return metAlertsCredit }

func (m *MetAlerts) Alerts(ctx context.Context, t Town, now time.Time) ([]Alert, error) {
	var found struct {
		Features []struct {
			Properties struct {
				ID          string `json:"id"`
				Event       string `json:"eventAwarenessName"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Instruction string `json:"instruction"`
				Consequence string `json:"consequences"`
				Area        string `json:"area"`
				Level       string `json:"awareness_level"`
				Domain      string `json:"geographicDomain"`
				Status      string `json:"status"`
				Type        string `json:"type"`
				Ends        string `json:"eventEndingTime"`
			} `json:"properties"`
			When struct {
				Interval []string `json:"interval"`
			} `json:"when"`
		} `json:"features"`
	}
	u := fmt.Sprintf("%s/weatherapi/metalerts/2.0/current.json?lang=en&lat=%.2f&lon=%.2f", m.Base, t.Lat, t.Lon)
	if _, err := m.F.getJSON(ctx, u, "application/json", &found); err != nil {
		return nil, err
	}
	alerts := []Alert{}
	for _, f := range found.Features {
		p := f.Properties
		// At sea, for boats: not the town's.
		if p.Domain == "marine" || p.Status != "Actual" || p.Type == "Cancel" {
			continue
		}
		ends, starts := p.Ends, ""
		if len(f.When.Interval) == 2 {
			starts = f.When.Interval[0]
			if ends == "" {
				ends = f.When.Interval[1]
			}
		}
		if end, err := time.Parse(time.RFC3339, ends); err == nil && !end.After(now) {
			continue
		}
		level := levelOf(p.Level)
		event := fmt.Sprintf("%s (%s)", capital(p.Event), level)
		text := strings.TrimSpace(p.Description + " " + p.Consequence)
		alerts = append(alerts, Alert{
			ID: p.ID, Kind: kindForLevel(level), Level: level, Event: same(event), Headline: same(p.Title),
			Text: same(text), Instruction: same(p.Instruction), Area: same(p.Area), Starts: starts, Ends: ends,
		})
	}
	return alerts, nil
}

// levelOf: "2; yellow; Moderate" → "yellow".
func levelOf(awareness string) string {
	parts := strings.Split(awareness, ";")
	if len(parts) >= 2 {
		return strings.ToLower(strings.TrimSpace(parts[1]))
	}
	return ""
}

// kindForLevel: orange and red are warnings; yellow, be aware.
func kindForLevel(level string) string {
	switch level {
	case "orange", "red":
		return "warning"
	case "yellow":
		return "advisory"
	}
	return "statement"
}

// --- EUMETNET – MeteoAlarm ---------------------------------------------------------------

type MeteoAlarm struct {
	F    *Fetcher
	Base string // https://feeds.meteoalarm.org

	mu    sync.Mutex
	cache map[string]capAlert // by the CAP link, without its query
}

var meteoAlarmCredit = Words{"Weather warnings: EUMETNET – MeteoAlarm (meteoalarm.org), CC BY 4.0.",
	"Avertissements météo : EUMETNET – MeteoAlarm (meteoalarm.org), CC BY 4.0."}

func (m *MeteoAlarm) Credit() Words { return meteoAlarmCredit }

func (m *MeteoAlarm) Official(t Town) (Words, Words) {
	return same("https://meteoalarm.org?region=" + t.Country), Words{"MeteoAlarm's warnings (meteoalarm.org)", "les avertissements de MeteoAlarm (meteoalarm.org)"}
}

// meteoAlarmFeeds: each country MeteoAlarm has a feed for, its name there.
// Norway's own come from MetAlerts.
var meteoAlarmFeeds = map[string]string{
	"AD": "andorra", "AT": "austria", "BE": "belgium", "BA": "bosnia-herzegovina", "BG": "bulgaria",
	"HR": "croatia", "CY": "cyprus", "CZ": "czechia", "DK": "denmark", "EE": "estonia", "FI": "finland",
	"FR": "france", "DE": "germany", "GR": "greece", "HU": "hungary", "IS": "iceland", "IE": "ireland",
	"IL": "israel", "IT": "italy", "LV": "latvia", "LT": "lithuania", "LU": "luxembourg", "MT": "malta",
	"MD": "moldova", "ME": "montenegro", "NL": "netherlands", "PL": "poland", "PT": "portugal",
	"MK": "republic-of-north-macedonia", "RO": "romania", "RS": "serbia", "SK": "slovakia",
	"SI": "slovenia", "ES": "spain", "SE": "sweden", "CH": "switzerland", "UA": "ukraine",
	"GB": "united-kingdom",
}

type atomFeed struct {
	Entries []struct {
		Identifier string `xml:"identifier"`
		Status     string `xml:"status"`
		MsgType    string `xml:"message_type"`
		Geocodes   []struct {
			Name  string `xml:"valueName"`
			Value string `xml:"value"`
		} `xml:"geocode"`
		Links []struct {
			Type string `xml:"type,attr"`
			Href string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

type capInfo struct {
	Language    string `xml:"language"`
	Event       string `xml:"event"`
	Headline    string `xml:"headline"`
	Description string `xml:"description"`
	Instruction string `xml:"instruction"`
	Onset       string `xml:"onset"`
	Effective   string `xml:"effective"`
	Expires     string `xml:"expires"`
	Parameters  []struct {
		Name  string `xml:"valueName"`
		Value string `xml:"value"`
	} `xml:"parameter"`
	Areas []struct {
		Desc     string   `xml:"areaDesc"`
		Polygons []string `xml:"polygon"`
		Geocodes []struct {
			Name  string `xml:"valueName"`
			Value string `xml:"value"`
		} `xml:"geocode"`
	} `xml:"area"`
}

type capAlert struct {
	Identifier string    `xml:"identifier"`
	Sent       string    `xml:"sent"`
	Status     string    `xml:"status"`
	MsgType    string    `xml:"msgType"`
	Infos      []capInfo `xml:"info"`
}

// The most new CAP documents read in one round; the rest wait for the next.
const capPerRound = 40

func (m *MeteoAlarm) Alerts(ctx context.Context, t Town, now time.Time) ([]Alert, error) {
	name, ok := meteoAlarmFeeds[t.Country]
	if !ok {
		return []Alert{}, nil
	}
	a, err := m.F.Get(ctx, m.Base+"/feeds/meteoalarm-legacy-atom-"+name, "", "")
	if err != nil {
		return nil, err
	}
	var feed atomFeed
	if err := xml.Unmarshal(a.Body, &feed); err != nil {
		return nil, fmt.Errorf("MeteoAlarm's feed didn't read")
	}
	mine := map[string]bool{}
	for _, code := range t.Areas {
		mine[code] = true
	}
	// The CAP documents to look at: a coded entry only when the code is the
	// town's; an entry without a code, to see its polygons.
	wanted := []string{}
	seen := map[string]bool{}
	for _, e := range feed.Entries {
		if e.Status != "" && e.Status != "Actual" {
			continue
		}
		coded, ours := false, false
		for _, g := range e.Geocodes {
			if g.Name == "EMMA_ID" {
				coded = true
				ours = ours || mine[g.Value]
			}
		}
		if coded && !ours {
			continue
		}
		for _, l := range e.Links {
			if l.Type != "application/cap+xml" {
				continue
			}
			link := l.Href
			if i := strings.IndexByte(link, '?'); i >= 0 {
				link = link[:i]
			}
			if !seen[link] {
				seen[link] = true
				wanted = append(wanted, link)
			}
		}
	}
	m.mu.Lock()
	if m.cache == nil {
		m.cache = map[string]capAlert{}
	}
	for link := range m.cache {
		if !seen[link] {
			delete(m.cache, link) // gone from the feed
		}
	}
	m.mu.Unlock()
	read := 0
	alerts := []Alert{}
	byID := map[string]bool{}
	for _, link := range wanted {
		m.mu.Lock()
		cap, cached := m.cache[link]
		m.mu.Unlock()
		if !cached {
			if read == capPerRound {
				continue
			}
			read++
			doc, err := m.F.Get(ctx, link, "", "")
			if err != nil || xml.Unmarshal(doc.Body, &cap) != nil {
				continue
			}
			m.mu.Lock()
			m.cache[link] = cap
			m.mu.Unlock()
		}
		if alert, ok := capFor(cap, t, mine, now); ok && !byID[alert.ID] {
			byID[alert.ID] = true
			alerts = append(alerts, alert)
		}
	}
	sort.Slice(alerts, func(i, j int) bool { return alerts[i].Ends < alerts[j].Ends })
	return alerts, nil
}

// capFor: the CAP alert as one of ours, when one of its areas is the town's.
func capFor(c capAlert, t Town, mine map[string]bool, now time.Time) (Alert, bool) {
	if c.Status != "Actual" || c.MsgType == "Cancel" || len(c.Infos) == 0 {
		return Alert{}, false
	}
	area := ""
	for _, info := range c.Infos {
		for _, a := range info.Areas {
			if area != "" {
				break
			}
			for _, g := range a.Geocodes {
				if g.Name == "EMMA_ID" && mine[g.Value] {
					area = a.Desc
				}
			}
			for _, p := range a.Polygons {
				if area == "" && capPolygonHolds(p, t.Lat, t.Lon) {
					area = a.Desc
				}
			}
		}
	}
	if area == "" {
		return Alert{}, false
	}
	en, fr := pickInfo(c.Infos, "en"), pickInfo(c.Infos, "fr")
	ends := en.Expires
	if end, err := time.Parse(time.RFC3339, ends); err == nil && !end.After(now) {
		return Alert{}, false
	}
	level := ""
	for _, p := range en.Parameters {
		if p.Name == "awareness_level" {
			level = levelOf(p.Value)
		}
	}
	starts := en.Onset
	if starts == "" {
		starts = en.Effective
	}
	return Alert{
		ID: c.Identifier, Kind: kindForLevel(level), Level: level,
		Event:       Words{capital(en.Event), capital(fr.Event)},
		Headline:    Words{en.Headline, fr.Headline},
		Text:        Words{en.Description, fr.Description},
		Instruction: Words{en.Instruction, fr.Instruction},
		Area:        same(area), Issued: c.Sent, Starts: starts, Ends: ends,
	}, true
}

// pickInfo: the info in that language, else English, else the first.
func pickInfo(infos []capInfo, lang string) capInfo {
	for _, want := range []string{lang, "en"} {
		for _, i := range infos {
			if strings.HasPrefix(strings.ToLower(i.Language), want) {
				return i
			}
		}
	}
	return infos[0]
}

// capPolygonHolds: CAP's polygon, "lat,lon lat,lon …", holds the point.
func capPolygonHolds(polygon string, lat, lon float64) bool {
	ring := [][]float64{}
	for _, pair := range strings.Fields(polygon) {
		la, lo, ok := strings.Cut(pair, ",")
		if !ok {
			return false
		}
		y, err1 := strconv.ParseFloat(la, 64)
		x, err2 := strconv.ParseFloat(lo, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		ring = append(ring, []float64{x, y})
	}
	return len(ring) >= 4 && inRing(ring, lat, lon)
}
