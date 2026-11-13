// Control Panel › Language & Region and Date and time: the languages our
// screens come in, the countries and their states or provinces (named in the
// admin's language, as Debian's iso-codes translates them), the time zones,
// and the locale for a language in a region. The same lists and rules as the
// first setup's (zoomriya_setup/app.py), from the same files on the box.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Places reads the box's files; Root is "/" on the box.
type Places struct {
	Root string

	once        sync.Once
	isoNames    map[string]string // alpha-2: ISO English name
	subdivision []isoSubdivision
	locales     []string
	zones       []string
	mu          sync.Mutex
	catalogs    map[string]map[string]string // domain/lang: English: translation
}

type isoSubdivision struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Parent string `json:"parent"`
}

// Choice is a code and its name, as a dropdown lists it.
type Choice struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Languages: those our screens are translated into, each in itself
// (zoomriya_setup.app.languages).
var Languages = []Choice{{"en", "English"}, {"fr", "Français"}, {"de", "Deutsch"}, {"es", "Español"},
	{"ta", "தமிழ்"}, {"ja", "日本語"}, {"zh", "简体中文"}}

// ourLanguages are gettext's codes for them (zoomriya_setup.i18n.OURS).
var ourLanguages = []string{"en", "fr", "de", "es", "ta", "ja", "zh_CN"}

// Ours: a language as Debian names it, as our screens show it; English for
// the rest (zoomriya_setup.i18n.ours).
func Ours(code string) string {
	code, _, _ = strings.Cut(code, ".")
	for _, l := range ourLanguages {
		if l == code {
			return code
		}
	}
	base, _, _ := strings.Cut(code, "_")
	if base == "zh" {
		return "zh_CN"
	}
	for _, l := range ourLanguages {
		if l == base {
			return base
		}
	}
	return "en"
}

// homeLocales: a language's home locale, where it isn't xx_XX (app.py's HOME_LOCALES).
var homeLocales = map[string]string{
	"ar": "ar_EG", "bn": "bn_BD", "ca": "ca_ES", "cs": "cs_CZ", "da": "da_DK", "el": "el_GR",
	"en": "en_US", "et": "et_EE", "eu": "eu_ES", "fa": "fa_IR", "ga": "ga_IE", "gl": "gl_ES",
	"gu": "gu_IN", "he": "he_IL", "hi": "hi_IN", "hy": "hy_AM", "ja": "ja_JP", "ka": "ka_GE",
	"kk": "kk_KZ", "kn": "kn_IN", "ko": "ko_KR", "ml": "ml_IN", "mr": "mr_IN", "ms": "ms_MY",
	"nb": "nb_NO", "ne": "ne_NP", "nn": "nn_NO", "pa": "pa_IN", "si": "si_LK", "sl": "sl_SI",
	"sq": "sq_AL", "sv": "sv_SE", "sw": "sw_KE", "ta": "ta_IN", "te": "te_IN", "uk": "uk_UA",
	"ur": "ur_PK", "vi": "vi_VN", "zh": "zh_CN",
}

func (p *Places) path(rel string) string { return filepath.Join(p.Root, rel) }

func (p *Places) load() {
	p.once.Do(func() {
		p.catalogs = map[string]map[string]string{}
		p.isoNames = map[string]string{}
		var countries struct {
			List []struct {
				Alpha2 string `json:"alpha_2"`
				Name   string `json:"name"`
			} `json:"3166-1"`
		}
		if raw, err := os.ReadFile(p.path("usr/share/iso-codes/json/iso_3166-1.json")); err == nil {
			json.Unmarshal(raw, &countries)
		}
		for _, c := range countries.List {
			p.isoNames[c.Alpha2] = c.Name
		}
		var subs struct {
			List []isoSubdivision `json:"3166-2"`
		}
		if raw, err := os.ReadFile(p.path("usr/share/iso-codes/json/iso_3166-2.json")); err == nil {
			json.Unmarshal(raw, &subs)
		}
		p.subdivision = subs.List
		// The locales Debian can build in UTF-8, without variants, nor C.UTF-8.
		for _, row := range tabRows(p.path("usr/share/i18n/SUPPORTED")) {
			fields := strings.Fields(row[0])
			if len(fields) == 2 && fields[1] == "UTF-8" && !strings.Contains(fields[0], "@") &&
				!strings.HasPrefix(fields[0], "C.") {
				p.locales = append(p.locales, fields[0])
			}
		}
		p.zones = timeZones(p.path("usr/share/zoneinfo"))
	})
}

// tabRows: a tab-separated file's rows, without comments.
func tabRows(path string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows [][]string
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

// timeZones: places people pick (Area/City), not the database's aliases, and UTC.
func timeZones(root string) []string {
	areas := map[string]bool{"Africa": true, "America": true, "Antarctica": true, "Asia": true, "Atlantic": true,
		"Australia": true, "Europe": true, "Indian": true, "Pacific": true}
	var zones []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		area, _, _ := strings.Cut(rel, string(filepath.Separator))
		if d.IsDir() {
			if rel != "." && !areas[area] {
				return filepath.SkipDir
			}
			return nil
		}
		if !areas[area] || strings.Contains(rel, ".") {
			return nil
		}
		// A zone file starts with TZif.
		if f, err := os.Open(path); err == nil {
			head := make([]byte, 4)
			n, _ := f.Read(head)
			f.Close()
			if n == 4 && string(head) == "TZif" {
				zones = append(zones, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(zones)
	return append(zones, "UTC")
}

func byName(list []Choice) {
	sort.SliceStable(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
}

// translate: an English name in lang, from Debian's catalogue for domain.
func (p *Places) translate(domain, lang, english string) string {
	if lang == "en" {
		return english
	}
	key := domain + "/" + lang
	p.mu.Lock()
	catalog, ok := p.catalogs[key]
	if !ok {
		catalog = readMO(p.path("usr/share/locale/" + lang + "/LC_MESSAGES/" + domain + ".mo"))
		p.catalogs[key] = catalog
	}
	p.mu.Unlock()
	if t := catalog[english]; t != "" {
		return t
	}
	return english
}

// readMO reads a GNU .mo catalogue: its originals and their translations.
func readMO(path string) map[string]string {
	found := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 28 {
		return found
	}
	var order binary.ByteOrder = binary.LittleEndian
	switch order.Uint32(data) {
	case 0x950412de:
	case 0xde120495:
		order = binary.BigEndian
	default:
		return found
	}
	count := int(order.Uint32(data[8:]))
	ids, strs := int(order.Uint32(data[12:])), int(order.Uint32(data[16:]))
	text := func(table, i int) (string, bool) {
		at := table + 8*i
		if at+8 > len(data) {
			return "", false
		}
		n, off := int(order.Uint32(data[at:])), int(order.Uint32(data[at+4:]))
		if off+n > len(data) || off < 0 || n < 0 {
			return "", false
		}
		return string(data[off : off+n]), true
	}
	for i := 0; i < count; i++ {
		id, ok1 := text(ids, i)
		str, ok2 := text(strs, i)
		if ok1 && ok2 && id != "" {
			// Plural forms: the first of each.
			id, _, _ = strings.Cut(id, "\x00")
			str, _, _ = strings.Cut(str, "\x00")
			found[id] = str
		}
	}
	return found
}

// Regions: countries and regions as tzdata lists them, named in lang, by name.
func (p *Places) Regions(lang string) []Choice {
	p.load()
	var list []Choice
	for _, row := range tabRows(p.path("usr/share/zoneinfo/iso3166.tab")) {
		if len(row) < 2 {
			continue
		}
		name := row[1]
		if iso, ok := p.isoNames[row[0]]; ok {
			name = p.translate("iso_3166-1", lang, iso)
		}
		list = append(list, Choice{row[0], name})
	}
	byName(list)
	return list
}

// Subdivisions: a country's top-level states or provinces, named in lang, by name.
func (p *Places) Subdivisions(region, lang string) []Choice {
	p.load()
	list := []Choice{}
	for _, s := range p.subdivision {
		if strings.HasPrefix(s.Code, region+"-") && s.Parent == "" {
			list = append(list, Choice{s.Code, p.translate("iso_3166-2", lang, s.Name)})
		}
	}
	byName(list)
	return list
}

// ZonesIn: a country's own time zones, as tzdata lists them (zone.tab).
func (p *Places) ZonesIn(region string) []string {
	zones := []string{}
	for _, row := range tabRows(p.path("usr/share/zoneinfo/zone.tab")) {
		if len(row) > 2 && row[0] == region {
			zones = append(zones, row[2])
		}
	}
	sort.Strings(zones)
	return zones
}

func (p *Places) TimeZones() []string {
	p.load()
	return p.zones
}

// LocaleFor: the language as written in the region (ta_LK), else its home
// locale, else its first (app.py's locale_for).
func (p *Places) LocaleFor(language, region string) string {
	p.load()
	var mine []string
	for _, l := range p.locales {
		if base, _, _ := strings.Cut(l, "_"); base == language {
			mine = append(mine, l)
		}
	}
	if len(mine) == 0 {
		return ""
	}
	for _, l := range mine {
		if name, _, _ := strings.Cut(l, "."); name == language+"_"+region {
			return l
		}
	}
	home := homeLocales[language]
	if home == "" {
		home = language + "_" + strings.ToUpper(language)
	}
	for _, l := range mine {
		if name, _, _ := strings.Cut(l, "."); name == home {
			return l
		}
	}
	sort.Strings(mine)
	return mine[0]
}

// BoxLanguage: the box's language as Debian keeps it (LANG in
// /etc/default/locale), as our screens show it.
func (p *Places) BoxLanguage() string {
	if v := os.Getenv("BOX_DESKTOP_LANGUAGE"); v != "" {
		return Ours(v)
	}
	raw, err := os.ReadFile(p.path("etc/default/locale"))
	if err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if v, ok := strings.CutPrefix(line, "LANG="); ok {
				return Ours(strings.Trim(strings.TrimSpace(v), `"`))
			}
		}
	}
	return "en"
}

// OSName: the OS's name as os-release gives it; just "ZOOMRIYA OS" on a
// computer that isn't a box (app.py's os_name).
func (p *Places) OSName() string {
	raw, err := os.ReadFile(p.path("usr/lib/os-release"))
	if err != nil {
		return "ZOOMRIYA OS"
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			fields[k] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	if fields["ID"] != "zoomriya" || fields["PRETTY_NAME"] == "" {
		return "ZOOMRIYA OS"
	}
	return fields["PRETTY_NAME"]
}

// Processor: its name and how many threads it runs, from the kernel (control.py's processor).
func (p *Places) Processor() string {
	raw, err := os.ReadFile(p.path("proc/cpuinfo"))
	if err != nil {
		return ""
	}
	name, threads := "", 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "model name") {
			if _, v, ok := strings.Cut(line, ":"); ok && name == "" {
				name = strings.TrimSpace(v)
			}
		}
		if strings.HasPrefix(line, "processor") {
			threads++
		}
	}
	if name == "" {
		return ""
	}
	return name + " × " + strconv.Itoa(threads)
}
