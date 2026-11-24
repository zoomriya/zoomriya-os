package main

import (
	"bytes"
	"compress/gzip"
	"testing"
)

func writeTowns(t *testing.T, box *fakeBox) {
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	z.Write([]byte("# id\tname\tascii\tcountry\tadmin1\tlat\tlon\tpopulation\ttimezone\n" +
		"6167865\tToronto\tToronto\tCA\tOntario\t43.6533\t-79.3839\t2794356\tAmerica/Toronto\n" +
		"5128581\tNew York City\tNew York City\tUS\tNew York\t40.7143\t-74.0060\t8804190\tAmerica/New_York\t\n" +
		"2988507\tParis\tParis\tFR\tÎle-de-France\t48.8534\t2.3488\t2138551\tEurope/Paris\tFR067,FR101\n"))
	z.Close()
	box.writeBytes(townsFile, buf.Bytes())
}

func TestTown(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("usr/share/zoneinfo/iso3166.tab", "CA\tCanada\nUS\tUnited States\nFR\tFrance\n")
	writeTowns(t, box)
	if resp, _ := agent.Handle(Request{Op: "setup-status"}); resp["town"] != (*Town)(nil) {
		t.Fatalf("a town before one was chosen: %v", resp["town"])
	}
	id := int64(6167865)
	if _, err := agent.Handle(Request{Op: "town-set", Town: &id}); err == nil {
		t.Fatal("a town before the country")
	}
	agent.Handle(Request{Op: "region-set", Region: str("CA")})
	if _, err := agent.Handle(Request{Op: "town-set", Town: &id}); err != nil {
		t.Fatal(err)
	}
	got := agent.town()
	if got == nil || got.Name != "Toronto" || got.Country != "CA" || got.Lat != 43.6533 || got.TimeZone != "America/Toronto" {
		t.Fatalf("%+v", got)
	}
	// A town in another country, or not on the list: refused.
	for _, other := range []int64{5128581, 42, -1} {
		if _, err := agent.Handle(Request{Op: "town-set", Town: &other}); err == nil {
			t.Errorf("%d accepted", other)
		}
	}
	// A European town keeps its MeteoAlarm areas.
	agent.Handle(Request{Op: "region-set", Region: str("FR")})
	paris := int64(2988507)
	if _, err := agent.Handle(Request{Op: "town-set", Town: &paris}); err != nil {
		t.Fatal(err)
	}
	if got := agent.town(); got == nil || len(got.Areas) != 2 || got.Areas[1] != "FR101" {
		t.Fatalf("%+v", got)
	}
	agent.Handle(Request{Op: "region-set", Region: str("CA")})
	agent.Handle(Request{Op: "town-set", Town: &id})
	// The same country again keeps it; another country forgets it.
	agent.Handle(Request{Op: "region-set", Region: str("CA")})
	if agent.town() == nil {
		t.Fatal("forgotten for the same country")
	}
	agent.Handle(Request{Op: "region-set", Region: str("US")})
	if agent.town() != nil {
		t.Fatal("a Canadian town kept for a box in the US")
	}
}
