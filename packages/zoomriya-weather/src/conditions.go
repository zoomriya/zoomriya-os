package main

import (
	"math"
	"strings"
)

// The icons Box Desktop and the apps draw, one set for every service.
const (
	IconClear   = "clear"
	IconPartly  = "partly"
	IconCloudy  = "cloudy"
	IconFog     = "fog"
	IconRain    = "rain"
	IconShowers = "showers"
	IconSleet   = "sleet"
	IconSnow    = "snow"
	IconStorm   = "storm"
)

// iconFor reads a service's English words for the sky ("Mainly sunny",
// "Chance of showers", "Light snow and fog").
func iconFor(words string) string {
	w := strings.ToLower(words)
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(w, p) {
				return true
			}
		}
		return false
	}
	switch {
	case has("thunder", "storm"):
		return IconStorm
	case has("freezing", "sleet", "ice pellet", "wintry mix", "rain and snow", "snow and rain"):
		return IconSleet
	case has("snow", "flurr", "blizzard"):
		return IconSnow
	case has("shower"):
		return IconShowers
	case has("rain", "drizzle"):
		return IconRain
	case has("fog", "mist", "haze", "smoke"):
		return IconFog
	case has("partly", "mix of sun", "mainly sunny", "mainly clear", "few clouds", "mostly sunny",
		"mostly clear", "a few", "clearing", "variable cloudiness"):
		return IconPartly
	case has("cloud", "overcast"):
		return IconCloudy
	case has("sun", "clear", "fair"):
		return IconClear
	}
	return IconCloudy
}

// metSymbols: MET Norway's symbol codes (without _day, _night or
// _polartwilight), in our words and icons.
var metSymbols = map[string]struct {
	Words
	Icon string
}{
	"clearsky":                   {Words{"Clear", "Dégagé"}, IconClear},
	"fair":                       {Words{"Mainly clear", "Généralement dégagé"}, IconPartly},
	"partlycloudy":               {Words{"Partly cloudy", "Partiellement nuageux"}, IconPartly},
	"cloudy":                     {Words{"Cloudy", "Nuageux"}, IconCloudy},
	"fog":                        {Words{"Fog", "Brouillard"}, IconFog},
	"lightrain":                  {Words{"Light rain", "Pluie faible"}, IconRain},
	"rain":                       {Words{"Rain", "Pluie"}, IconRain},
	"heavyrain":                  {Words{"Heavy rain", "Forte pluie"}, IconRain},
	"lightrainshowers":           {Words{"Light showers", "Faibles averses"}, IconShowers},
	"rainshowers":                {Words{"Showers", "Averses"}, IconShowers},
	"heavyrainshowers":           {Words{"Heavy showers", "Fortes averses"}, IconShowers},
	"lightsleet":                 {Words{"Light sleet", "Faible neige fondante"}, IconSleet},
	"sleet":                      {Words{"Sleet", "Neige fondante"}, IconSleet},
	"heavysleet":                 {Words{"Heavy sleet", "Forte neige fondante"}, IconSleet},
	"lightsleetshowers":          {Words{"Light sleet showers", "Faibles averses de neige fondante"}, IconSleet},
	"sleetshowers":               {Words{"Sleet showers", "Averses de neige fondante"}, IconSleet},
	"heavysleetshowers":          {Words{"Heavy sleet showers", "Fortes averses de neige fondante"}, IconSleet},
	"lightsnow":                  {Words{"Light snow", "Faible neige"}, IconSnow},
	"snow":                       {Words{"Snow", "Neige"}, IconSnow},
	"heavysnow":                  {Words{"Heavy snow", "Forte neige"}, IconSnow},
	"lightsnowshowers":           {Words{"Light flurries", "Faibles averses de neige"}, IconSnow},
	"snowshowers":                {Words{"Flurries", "Averses de neige"}, IconSnow},
	"heavysnowshowers":           {Words{"Heavy flurries", "Fortes averses de neige"}, IconSnow},
	"lightrainandthunder":        {Words{"Light rain and thunder", "Pluie faible et orages"}, IconStorm},
	"rainandthunder":             {Words{"Rain and thunder", "Pluie et orages"}, IconStorm},
	"heavyrainandthunder":        {Words{"Heavy rain and thunder", "Forte pluie et orages"}, IconStorm},
	"lightrainshowersandthunder": {Words{"Showers and thunder", "Averses et orages"}, IconStorm},
	"rainshowersandthunder":      {Words{"Showers and thunder", "Averses et orages"}, IconStorm},
	"heavyrainshowersandthunder": {Words{"Heavy showers and thunder", "Fortes averses et orages"}, IconStorm},
}

// metSymbol reads one of MET Norway's codes; an unknown one by its parts.
func metSymbol(code string) (Words, string) {
	base := code
	for _, suffix := range []string{"_day", "_night", "_polartwilight"} {
		base = strings.TrimSuffix(base, suffix)
	}
	if found, ok := metSymbols[base]; ok {
		return found.Words, found.Icon
	}
	switch {
	case strings.Contains(base, "thunder"):
		return Words{"Thunderstorms", "Orages"}, IconStorm
	case strings.Contains(base, "snow"):
		return Words{"Snow", "Neige"}, IconSnow
	case strings.Contains(base, "sleet"):
		return Words{"Sleet", "Neige fondante"}, IconSleet
	case strings.Contains(base, "rain"):
		return Words{"Rain", "Pluie"}, IconRain
	}
	return Words{"Cloudy", "Nuageux"}, IconCloudy
}

// Temperatures: Celsius everywhere but the US (weather.md W-5).
func toF(c float64) float64 { return c*9/5 + 32 }
func toC(f float64) float64 { return (f - 32) * 5 / 9 }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func ptr[T any](v T) *T { return &v }

// The days' names when a service doesn't give them (MET Norway).
var weekdays = [7]Words{
	{"Sunday", "dimanche"}, {"Monday", "lundi"}, {"Tuesday", "mardi"}, {"Wednesday", "mercredi"},
	{"Thursday", "jeudi"}, {"Friday", "vendredi"}, {"Saturday", "samedi"},
}
