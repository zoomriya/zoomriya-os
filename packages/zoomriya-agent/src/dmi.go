// The box's model and serial, from the firmware's DMI tables (About this box).
// Mini PCs often leave them as the board maker's placeholders; those count as
// none, so the box never shows "To be filled by O.E.M." as its serial.
package main

import "strings"

const dmiDir = "sys/class/dmi/id"

var dmiPlaceholders = []string{
	"", "0", "none", "default string", "to be filled by o.e.m.", "system product name",
	"system manufacturer", "not specified", "not applicable", "123456789", "o.e.m.",
}

func (a *Agent) dmi(name string) string {
	value := strings.TrimSpace(a.readTrimmed(dmiDir + "/" + name))
	for _, p := range dmiPlaceholders {
		if strings.EqualFold(value, p) {
			return ""
		}
	}
	return value
}

func (a *Agent) model() string {
	vendor, product := a.dmi("sys_vendor"), a.dmi("product_name")
	if vendor != "" && product != "" && !strings.HasPrefix(strings.ToLower(product), strings.ToLower(vendor)) {
		return vendor + " " + product
	}
	if product != "" {
		return product
	}
	return vendor
}

func (a *Agent) serial() string {
	return a.dmi("product_serial")
}
