// town-set (weather.md §1, §4): the family's town, chosen in the first
// setup after the country, or in Control Panel later, from GeoNames' towns
// shipped with the OS (zoomriya-weather's towns.tsv.gz). Kept in
// /etc/zoomriya/town beside the region, for the box's weather. The image has
// none: until one is chosen, weather fetches nothing.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

const (
	townFile  = "etc/zoomriya/town"
	townsFile = "usr/share/zoomriya/towns.tsv.gz"
)

// Town is one line of the towns file, as the weather service reads it.
type Town struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Admin1   string  `json:"admin1,omitempty"`
	Country  string  `json:"country"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	TimeZone string  `json:"timezone,omitempty"`
	// Areas: the MeteoAlarm areas the town lies in (Europe), for its alerts.
	Areas []string `json:"areas,omitempty"`
}

// town is the chosen town; nil when none is.
func (a *Agent) town() *Town {
	data, err := os.ReadFile(a.path(townFile))
	if err != nil {
		return nil
	}
	var t Town
	if json.Unmarshal(data, &t) != nil || t.ID == 0 {
		return nil
	}
	return &t
}

// findTown: the town with GeoNames id `id`, from the shipped list.
func (a *Agent) findTown(id int64) (*Town, error) {
	f, err := os.Open(a.path(townsFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	want := strconv.FormatInt(id, 10) + "\t"
	lines := bufio.NewScanner(z)
	lines.Buffer(make([]byte, 64<<10), 1<<20)
	for lines.Scan() {
		line := lines.Text()
		if !strings.HasPrefix(line, want) {
			continue
		}
		// id, name, ascii, country, admin1, lat, lon, population, time zone,
		// and in Europe its MeteoAlarm areas
		p := strings.Split(line, "\t")
		if len(p) < 9 {
			return nil, nil
		}
		lat, err1 := strconv.ParseFloat(p[5], 64)
		lon, err2 := strconv.ParseFloat(p[6], 64)
		if err1 != nil || err2 != nil {
			return nil, nil
		}
		t := &Town{ID: id, Name: p[1], Admin1: p[4], Country: p[3], Lat: lat, Lon: lon, TimeZone: p[8]}
		if len(p) > 9 && p[9] != "" {
			t.Areas = strings.Split(p[9], ",")
		}
		return t, nil
	}
	return nil, lines.Err()
}

// townSet keeps the town; it must be in the box's country.
func (a *Agent) townSet(id *int64) (Response, error) {
	if id == nil || *id <= 0 {
		return nil, refuse("Which town? Choose one from the list.")
	}
	region := a.region()
	if region == "" {
		return nil, refuse("Choose the country first.")
	}
	t, err := a.findTown(*id)
	if err != nil {
		return nil, err
	}
	if t == nil || t.Country != region {
		return nil, refuse("Which town? Choose one from the list.")
	}
	data, _ := json.MarshalIndent(t, "", "  ")
	if err := writeFileAtomic(a.path(townFile), append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	return Response{"town": t}, nil
}

// forgetTownElsewhere: a new country forgets a town in the old one.
func (a *Agent) forgetTownElsewhere(region string) error {
	if t := a.town(); t != nil && t.Country != region {
		if err := os.Remove(a.path(townFile)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
