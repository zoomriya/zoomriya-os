package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

// MetNo: MET Norway's location forecast (api.met.no), for a box anywhere
// but Canada and the US: an hourly forecast, worldwide, no alerts (its
// alerts cover Norway only; weather.md W-4). CC BY 4.0. Its terms ask for a
// User-Agent naming us, and for no new request before the last answer's
// Expires, with If-Modified-Since: the service keeps both.
type MetNo struct {
	F    *Fetcher
	Base string // https://api.met.no
}

var metCredit = Words{"Weather data from MET Norway, CC BY 4.0.", "Données météo de MET Norway, CC BY 4.0."}

type metAnswer struct {
	Properties struct {
		Timeseries []struct {
			Time string `json:"time"`
			Data struct {
				Instant struct {
					Details struct {
						Temp *float64 `json:"air_temperature"`
					} `json:"details"`
				} `json:"instant"`
				Next1 *struct {
					Summary struct {
						Symbol string `json:"symbol_code"`
					} `json:"summary"`
				} `json:"next_1_hours"`
				Next6 *struct {
					Summary struct {
						Symbol string `json:"symbol_code"`
					} `json:"summary"`
				} `json:"next_6_hours"`
			} `json:"data"`
		} `json:"timeseries"`
	} `json:"properties"`
}

// errNotModified: MET Norway's forecast hasn't changed since the last one.
var errNotModified = fmt.Errorf("not modified")

func (m *MetNo) Forecast(ctx context.Context, t Town, lastModified string) (Forecast, error) {
	u := fmt.Sprintf("%s/weatherapi/locationforecast/2.0/compact?lat=%.2f&lon=%.2f", m.Base, t.Lat, t.Lon)
	a, err := m.F.Get(ctx, u, "application/json", lastModified)
	if err != nil {
		return Forecast{}, err
	}
	if a.Status == http.StatusNotModified {
		return Forecast{Expires: a.Expires, LastModified: lastModified}, errNotModified
	}
	var met metAnswer
	if err := json.Unmarshal(a.Body, &met); err != nil {
		return Forecast{}, fmt.Errorf("MET Norway's answer didn't read")
	}
	f := Forecast{Unit: "C", Expires: a.Expires, LastModified: a.LastModified}
	zone := townZone(t)
	series := met.Properties.Timeseries
	if len(series) == 0 {
		return Forecast{}, fmt.Errorf("MET Norway's forecast had no hours")
	}
	type acc struct {
		day      *Day
		min, max float64
		noonGap  float64
	}
	byDate := map[string]*acc{}
	var order []string
	for i, s := range series {
		at, err := time.Parse(time.RFC3339, s.Time)
		if err != nil || s.Data.Instant.Details.Temp == nil {
			continue
		}
		temp := *s.Data.Instant.Details.Temp
		if i == 0 {
			symbol := ""
			if s.Data.Next1 != nil {
				symbol = s.Data.Next1.Summary.Symbol
			} else if s.Data.Next6 != nil {
				symbol = s.Data.Next6.Summary.Symbol
			}
			words, icon := metSymbol(symbol)
			f.Now = &Now{Time: s.Time, Temp: ptr(temp), Condition: words, Icon: icon}
		}
		local := at.In(zone)
		date := local.Format("2006-01-02")
		d := byDate[date]
		if d == nil {
			if len(order) == 7 {
				break
			}
			d = &acc{day: &Day{Date: date}, min: math.Inf(1), max: math.Inf(-1), noonGap: math.Inf(1)}
			byDate[date] = d
			order = append(order, date)
		}
		d.min, d.max = math.Min(d.min, temp), math.Max(d.max, temp)
		// The day's sky: the six hours from nearest midday.
		if s.Data.Next6 != nil {
			gap := math.Abs(float64(local.Hour()) - 12)
			if gap < d.noonGap {
				d.noonGap = gap
				words, icon := metSymbol(s.Data.Next6.Summary.Symbol)
				d.day.Condition, d.day.Icon = words, icon
			}
		}
	}
	today := time.Now().In(zone).Format("2006-01-02")
	tomorrow := time.Now().In(zone).AddDate(0, 0, 1).Format("2006-01-02")
	for _, date := range order {
		d := byDate[date]
		d.day.High, d.day.Low = ptr(d.max), ptr(d.min)
		day, _ := time.ParseInLocation("2006-01-02", date, zone)
		switch date {
		case today:
			d.day.Name = Words{"Today", "Aujourd'hui"}
		case tomorrow:
			d.day.Name = Words{"Tomorrow", "Demain"}
		default:
			d.day.Name = weekdays[day.Weekday()]
		}
		if d.day.Condition.En == "" {
			d.day.Condition, d.day.Icon = Words{"Cloudy", "Nuageux"}, IconCloudy
		}
		f.Days = append(f.Days, *d.day)
	}
	return f, nil
}
