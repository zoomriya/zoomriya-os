package main

import (
	"encoding/json"
	"math"
	"time"
)

func distanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := p2-p1, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * 6371.0088 * math.Asin(math.Sqrt(a))
}

// holds: the GeoJSON Polygon or MultiPolygon holds the point. A ring is
// [lon, lat] pairs; a point in a hole isn't held.
func holds(geometry json.RawMessage, lat, lon float64) bool {
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if json.Unmarshal(geometry, &g) != nil {
		return false
	}
	var polygons [][][][]float64
	switch g.Type {
	case "Polygon":
		var p [][][]float64
		if json.Unmarshal(g.Coordinates, &p) != nil {
			return false
		}
		polygons = [][][][]float64{p}
	case "MultiPolygon":
		if json.Unmarshal(g.Coordinates, &polygons) != nil {
			return false
		}
	default:
		return false
	}
	for _, p := range polygons {
		if len(p) == 0 || !inRing(p[0], lat, lon) {
			continue
		}
		hole := false
		for _, ring := range p[1:] {
			if inRing(ring, lat, lon) {
				hole = true
				break
			}
		}
		if !hole {
			return true
		}
	}
	return false
}

func inRing(ring [][]float64, lat, lon float64) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		if len(ring[i]) < 2 || len(ring[j]) < 2 {
			continue
		}
		xi, yi, xj, yj := ring[i][0], ring[i][1], ring[j][0], ring[j][1]
		if (yi > lat) != (yj > lat) && lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// townZone: the town's time zone (GeoNames'), else the box's.
func townZone(t Town) *time.Location {
	if t.TimeZone != "" {
		if z, err := time.LoadLocation(t.TimeZone); err == nil {
			return z
		}
	}
	return time.Local
}
