// language-set (design §5): the box's language and its country's ways of
// writing dates and numbers, as one of Debian's locales, such as ta_LK or
// en_CA.UTF-8. The box builds only the one chosen.
package main

import (
	"os"
	"regexp"
	"strings"
)

const (
	// Every locale Debian's locales package can build: "name charset".
	supportedFile = "usr/share/i18n/SUPPORTED"
	// The locales built; locale-gen builds them all again when locales is
	// updated, so the box's must be listed here.
	localeGenFile = "etc/locale.gen"
	// LANG, written by Debian's update-locale.
	localeFile = "etc/default/locale"
)

var localeName = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}(\.UTF-8)?(@[a-z]+)?$`)

// language is the box's locale; empty until the first setup asks.
func (a *Agent) language() string {
	data, err := os.ReadFile(a.path(localeFile))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "LANG="); found {
			value = strings.Trim(value, `"`)
			if value == "C.UTF-8" || value == "C" || value == "POSIX" {
				return ""
			}
			return value
		}
	}
	return ""
}

// supportedEntry is the locale's line in SUPPORTED, in UTF-8 only.
func (a *Agent) supportedEntry(locale string) string {
	data, err := os.ReadFile(a.path(supportedFile))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == locale && fields[1] == "UTF-8" {
			return line
		}
	}
	return ""
}

func (a *Agent) languageSet(locale *string) (Response, error) {
	if locale == nil || !localeName.MatchString(*locale) {
		return nil, refuse("Which language? Choose one from the list.")
	}
	entry := a.supportedEntry(*locale)
	if entry == "" {
		return nil, refuse("The box doesn't know a language called %s.", *locale)
	}
	if err := a.listLocale(entry); err != nil {
		return nil, err
	}
	if _, err := a.Run("locale-gen"); err != nil {
		return nil, err
	}
	if _, err := a.Run("update-locale", "LANG="+*locale); err != nil {
		return nil, err
	}
	return Response{"language": *locale}, nil
}

// listLocale adds the entry to locale.gen, or takes the # off its line.
func (a *Agent) listLocale(entry string) error {
	file := a.path(localeGenFile)
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry {
			return nil
		}
		if strings.TrimSpace(strings.TrimPrefix(trimmed, "#")) == entry {
			lines[i], found = entry, true
			break
		}
	}
	if !found {
		lines = append(lines, entry)
	}
	return os.WriteFile(file, []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n")+"\n"), 0o644)
}
