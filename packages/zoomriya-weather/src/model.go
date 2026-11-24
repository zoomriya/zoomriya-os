package main

// What the box keeps (weather.md §4): /var/lib/zoomriya-weather/weather.json,
// one shape whatever the service. Apps and Box Desktop read it; nothing
// writes it but this service.

// Words is a text in English and French. A service that writes only English
// (the NWS) gives the same words in both.
type Words struct {
	En string `json:"en"`
	Fr string `json:"fr"`
}

func same(text string) Words { return Words{text, text} }

// Town is the family's town, as the agent's town-set keeps it in
// /etc/zoomriya/town: one of GeoNames' towns shipped with the OS.
type Town struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Admin1   string  `json:"admin1,omitempty"`
	Country  string  `json:"country"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	TimeZone string  `json:"timezone,omitempty"`
	// Areas: the MeteoAlarm areas (EMMA_ID) the town lies in, for a town in
	// Europe; worked out when the OS is built.
	Areas []string `json:"areas,omitempty"`
}

// Now is the weather at the town now: observed at a station, or the
// service's own current conditions.
type Now struct {
	Time      string   `json:"time"`
	Temp      *float64 `json:"temp"`
	Condition Words    `json:"condition"`
	Icon      string   `json:"icon"`
	// Station: where it was observed, when the service says.
	Station Words `json:"station"`
}

// Day is one day of the forecast, in the town's own time.
type Day struct {
	Date      string   `json:"date"`
	Name      Words    `json:"name"`
	High      *float64 `json:"high"`
	Low       *float64 `json:"low"`
	Condition Words    `json:"condition"`
	Icon      string   `json:"icon"`
	// Chance: of precipitation, in percent, when the service gives one.
	Chance *int  `json:"chance,omitempty"`
	Text   Words `json:"text"`
	Night  Words `json:"night"`
}

// Alert is one of the weather service's alerts for the town.
type Alert struct {
	ID string `json:"id"`
	// Kind: warning, watch, advisory or statement.
	Kind string `json:"kind"`
	// Level: MeteoAlarm's and MET Norway's colour (yellow, orange, red),
	// when the service gives one.
	Level       string `json:"level,omitempty"`
	Event       Words  `json:"event"`
	Headline    Words  `json:"headline"`
	Text        Words  `json:"text"`
	Instruction Words  `json:"instruction"`
	Area        Words  `json:"area"`
	Issued      string `json:"issued,omitempty"`
	Starts      string `json:"starts,omitempty"`
	Ends        string `json:"ends,omitempty"`
}

// The states the file says it is in. No town chosen: no file at all.
const (
	// StateOK: the last fetch worked.
	StateOK = "ok"
	// StateStale: the last fetch failed; what's kept is from Fetched.
	StateStale = "stale"
	// StateWaiting: nothing fetched yet for this town.
	StateWaiting = "waiting"
	// StateOff: weather is switched off (its switch, Apps offline or
	// Everything offline); what's kept is from Fetched, if anything.
	StateOff = "off"
)

// Weather is the whole file.
type Weather struct {
	Version int    `json:"version"`
	State   string `json:"state"`
	Town    Town   `json:"town"`
	// Service: "eccc", "nws" or "metno".
	Service string `json:"service"`
	// Credit: shown with the weather, as each service's licence asks.
	Credit  Words  `json:"credit"`
	Licence string `json:"licence"`
	// Units: "C", or "F" for a box in the US.
	Units   string `json:"units"`
	Fetched string `json:"fetched,omitempty"`
	Expires string `json:"expires,omitempty"`
	// AlertsAvailable: the town's country has an alerts service we read
	// (Canada and the US); elsewhere the apps say alerts aren't available.
	AlertsAvailable bool `json:"alerts_available"`
	// AlertsService: where the alerts come from ("eccc", "nws", "metalerts",
	// "meteoalarm"); empty where there are none yet.
	AlertsService string  `json:"alerts_service,omitempty"`
	AlertsFetched string  `json:"alerts_fetched,omitempty"`
	Now           *Now    `json:"now,omitempty"`
	Days          []Day   `json:"days"`
	Alerts        []Alert `json:"alerts"`
	// Official: where the service's own warnings are, for "For official
	// warnings, see …".
	Official     Words `json:"official"`
	OfficialName Words `json:"official_name"`
	// Problem: why the last fetch failed, in plain words; empty when it worked.
	Problem string `json:"problem,omitempty"`
}

const fileVersion = 1
