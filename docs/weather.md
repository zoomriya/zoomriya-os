# Weather

The box's weather: the forecast and the weather alerts for the family's town,
fetched by the box from a government weather service and kept on the box for
every app to read. Moved here from the apps' documents on 16 November 2026 and
brought up to date (first written 1 November 2026).

Built on 16 November 2026: the service, with alerts in Canada, the US,
Norway and 37 more European countries; the town in the first setup and
Control Panel; the widget; Myira's answers; Family Hub's card and the
notices to every member (§11). Not built yet: alerts in the rest of the
world (W-4), and the phone app's notifications.

## 1. Decisions

| Decision | State |
| --- | --- |
| Weather is ZOOMRIYA OS's, box-wide like the time zone: one service of the OS fetches it, and every app reads what it kept (16 November 2026) | Decided |
| The weather service needs nothing from the apps: it works on a box with no apps, never calls one, reads none of their data and joins none of their groups. The apps get the weather from the box, and each decides what to show (16 November 2026) | Decided |
| No paid weather service: each country's government weather service, or MET Norway elsewhere, all free with commercial use allowed | Decided (1 November 2026) |
| Only the box talks to the weather services. Phones, laptops and browsers never do, and the apps never do | Decided (1 November 2026) |
| The family's town is chosen by the person installing, in the first setup's Town step right after the country, and must be filled in before Next. An admin changes it in Control Panel › Language & Region later; choosing another country forgets the old one's town | Decided (16 November 2026); built in the first setup, not the installer, which never asks the country |
| The OS image is built with no town: nothing is chosen at build time, and weather stays **not activated** (`zoomriya-weather` fetches nothing) until a town is chosen | Decided (16 November 2026) |
| Once the town is chosen, weather is on, and the same step says so: what leaves the box (the town, never anyone's name) and to whom. It has its own switch in Security › Internet Access | Decided (16 November 2026) |
| Alerts come from an official, free source that allows commercial use, for the family's one town only: Canada, the US, Norway and MeteoAlarm's European countries (§2). Elsewhere the forecast comes without alerts, and the box says "Weather alerts aren't available for your country yet." until we add a source for it (W-4) | Decided (16 November 2026) |
| Alerts are checked every 10 minutes, for the one town; never every country, never another town's | Decided (16 November 2026) |
| A Box Desktop widget shows the town's weather, so a box with no apps shows it too | Decided (16 November 2026) |
| Temperatures in Fahrenheit for a box in the US, in Celsius everywhere else (by the box's region) | Decided (16 November 2026) |
| Alerts reach the family three ways: Myira's answers, a notification to every member when a warning starts, and a card on Family Hub's dashboard while it lasts | Decided (16 November 2026); the three are the apps', each from what the box kept |
| Never a member's own location: not their phone's, not Family Tracker's | Decided (1 November 2026) |
| Not an emergency service: the box passes on what the weather service said, and says where official warnings are | Decided (1 November 2026) |

## 2. Where the weather comes from

### The forecast

| The family's country | Service | What the box asks |
| --- | --- | --- |
| Canada | Environment and Climate Change Canada (ECCC), its MSC GeoMet service | `https://api.weather.gc.ca/collections/citypageweather-realtime/items?bbox=…`: the city pages around the town; the nearest one's current conditions and forecast, in English and French |
| United States | The National Weather Service (NWS) | `https://api.weather.gov/points/<lat>,<lon>` once, then its forecast link, and the nearest station's latest observation (`/stations/<id>/observations/latest`) |
| Anywhere else | MET Norway | `https://api.met.no/weatherapi/locationforecast/2.0/compact?lat=…&lon=…`: an hourly forecast, worldwide, made into days in the town's own time |

### The alerts

Each source is official (a government weather service, or the network of
them), free, and allows commercial use (§3). The box asks only about the
family's one town, every 10 minutes.

| The family's country | Source | What the box asks | How the town's alerts are picked |
| --- | --- | --- | --- |
| Canada | ECCC, its MSC GeoMet weather alerts layer (`weather-alerts`, which replaced the experimental one in GeoMet 3.0, February 2026) | `https://api.weather.gc.ca/collections/weather-alerts/items?bbox=…`: the alerts within about 2 km of the town | Kept when the alert's polygon holds the town; one alert covering many areas kept once; ended or past its end left out. English and French, as ECCC writes them |
| United States | The NWS | `https://api.weather.gov/alerts/active?point=<lat>,<lon>`: the NWS picks those for the point | "Actual" alerts only, cancellations left out. English only |
| Norway | MET Norway's MetAlerts 2.0 | `https://api.met.no/weatherapi/metalerts/2.0/current.json?lang=en&lat=…&lon=…`: MET Norway picks those for the point | Marine alerts (for boats) left out; yellow kept as an advisory, orange and red as warnings. English |
| Andorra, Austria, Belgium, Bosnia and Herzegovina, Bulgaria, Croatia, Cyprus, Czechia, Denmark, Estonia, Finland, France, Germany, Greece, Hungary, Iceland, Ireland, Israel, Italy, Latvia, Lithuania, Luxembourg, Malta, Moldova, Montenegro, the Netherlands, North Macedonia, Poland, Portugal, Romania, Serbia, Slovakia, Slovenia, Spain, Sweden, Switzerland, Ukraine, the United Kingdom | EUMETNET – MeteoAlarm: each country's national weather service's warnings, gathered in one format | `https://feeds.meteoalarm.org/feeds/meteoalarm-legacy-atom-<country>`: the town's country's feed only; then, for each of its warnings for the town, its CAP document (`https://feeds.meteoalarm.org/api/v1/warnings/…`), once | A warning names its areas by MeteoAlarm's codes (EMMA_ID): kept when the code is one of the town's, which the OS works out when it is built from MeteoAlarm's published areas (§4), so no other area's warning is ever read. In the countries whose warnings carry no code (the UK, Sweden, Switzerland and others), each new warning's CAP is read once and kept only when one of its polygons holds the town. Yellow is an advisory, orange and red warnings. English, and French where the service writes it |
| Anywhere else | None yet (W-4) | Nothing | "Weather alerts aren't available for your country yet." |

Not used: GDACS (its terms say its alerts are automatic, unreviewed, and
not for decisions without confirmation; it is for disaster professionals),
and the IFRC Alert Hub until IFRC confirms its terms for reuse (W-4).

When the service doesn't answer, the box keeps the last forecast, marked with
when it was fetched (§5). It never switches to another service by itself.

## 3. Licences

Checked 16 November 2026; checked again before each release.

| Service | Licence | Commercial use | Credit shown with the weather |
| --- | --- | --- | --- |
| ECCC | ECCC's Data Servers End-use Licence, version 2.1 (September 2022), with its Service Usage Policy | Allowed, free, no account | "Contains information licenced under the Data Server End-use Licence of Environment and Climate Change Canada." |
| NWS | A US government work: public domain | Allowed, free, no account | "From the National Weather Service." Each request names ZOOMRIYA in its `User-Agent`, as NWS asks |
| MET Norway (the forecast) | Its terms of service; the data under CC BY 4.0 (and the Norwegian licence for open government data, NLOD) | Allowed, free, no account | "Weather data from MET Norway, CC BY 4.0." Each request names ZOOMRIYA in its `User-Agent`, and the box keeps to each answer's `Expires`, as its terms ask |
| MET Norway's MetAlerts | The same: CC BY 4.0 | Allowed, free, no account | "Weather alerts from MET Norway, CC BY 4.0." |
| EUMETNET – MeteoAlarm | Its terms and conditions: the warnings may be used and passed on under terms equivalent to CC BY 4.0, commercial use included; each warning stays its national service's | Allowed, free, no account (the Atom feeds; its other APIs need a token) | "Weather warnings: EUMETNET – MeteoAlarm (meteoalarm.org), CC BY 4.0.", with a link to meteoalarm.org, as its terms ask for more than one country's warnings in an internet application |

The credit and the licence's version are kept with each forecast, so whatever
shows it can show where it came from.

## 4. On the box

```
ECCC, NWS or MET Norway
        │  HTTPS, out only
        ▼
zoomriya-weather (the OS's, its own account)
        │  writes
        ▼
/var/lib/zoomriya-weather/weather.json (0644)
        │  read by
        ▼
Myira · Family Hub · notifications · Family Planner
```

| Piece | Plan |
| --- | --- |
| The service | `zoomriya-weather`: a small service of the OS, run as its own account `zoomriya-weather`. It reads the town, fetches, checks and keeps. It needs nothing from the apps: no access to `/data`, to any app's files, sockets, database or groups, or to ZOOMRIYA ID. It never calls an app or sends a notification; it only writes its own file |
| What it keeps | `/var/lib/zoomriya-weather/weather.json` (0644): the town, the service, when it was fetched and when it runs out, the forecast in one shape whatever the service, the active alerts, and the credit. Each new one is written whole, then moved into place, so nothing reads half of one |
| Who reads it | The apps get the weather from the box: they read the file, as they read where the family lives. It goes one way, box to apps. Nothing asks the service itself, so no app can make it fetch anything or reach its account |
| Internet | A declared connection, `/etc/zoomriya/outbound.d/weather.json`: the account `zoomriya-weather`, the hosts of the family's country's service only, optional ([Design §8.2](design.md#82-internet-access)). On by default; switched off there, or with **Apps offline**, the box keeps its last forecast and says how old it is |
| The town | Chosen by the person installing, in the first setup (`town-set`), from GeoNames' towns shipped with the OS (each with its coordinates, §9 W-1). Kept in `/etc/zoomriya/town` beside the region. The image has no `/etc/zoomriya/town`: until it exists, the service starts, finds no town and fetches nothing |

## 5. Fetching

| What | How often |
| --- | --- |
| Alerts | Every 10 minutes |
| The forecast | Every hour, or later when the answer's own `Expires` says so (MET Norway's always) |
| When it fails | The last good forecast stays. Tries again later, waiting longer each time (with some randomness), never more than one fetch at a time |
| An answer that doesn't check out | Refused: the wrong type, too large, times that don't read, numbers out of range. The last good forecast stays |
| A forecast past its time | Kept, shown as "Last updated 10:30", never as current |
| An alert past its end | Removed at once, even with no new answer |

Requests use HTTPS with the certificate checked, short timeouts and a size
limit. Nothing the services send is ever run or shown as HTML.

## 6. What leaves the box

| Goes out | Never goes out |
| --- | --- |
| The town: its coordinates rounded to 0.01° (about 1 km), to ECCC, the NWS and MET Norway. To MeteoAlarm, not even that: only a request for the country's whole feed, and for the documents of the warnings for the town | Anyone's name, account, phone or device |
| The box's internet address, as with any connection | A member's location, from their phone or Family Tracker |
| `User-Agent: ZOOMRIYA-OS/<version> (https://zoomriya.com)` | The family's photos, messages, or anything else |

The service logs only whether a fetch worked, how long it took and the
service's answer code: never the town or its coordinates.

## 7. What the apps show

| Where | What |
| --- | --- |
| Box Desktop (the OS's own) | A widget: the town, now, today and tomorrow, and any active warning, with the credit and when it was fetched. On a box with no apps too |
| Myira | "What's the weather tomorrow?", "Is there a warning?": the forecast and alerts as typed blocks, with the credit and when it was fetched. An active warning comes first in any weather answer. With weather off: what Myira says today, the usual weather from the climate normals |
| Notifications | Every member, when a warning starts for the town, and when it ends (watches, advisories and yellow levels are on the card without one). Not for every update of the same warning. Built in Family Hub, shown until the member presses Got it; on phones with the ZOOMRIYA phone app, later |
| Family Hub | A card at the top of the dashboard while a warning, watch or advisory lasts: what it is, its colour level where the source has one, until when, where the official warnings are, and the credit |
| Family Planner | Later: the forecast beside the day's events |

Each app reads `weather.json` and decides by itself: the notifications are
sent by the apps' own notifications, when they see a new alert in the file,
never by the weather service.

Each says it is the weather service's word, with where official warnings are:
"For official warnings, see Environment Canada's alerts." (or the NWS's).

## 8. When something is down

| Case | What happens |
| --- | --- |
| No internet | The last forecast, with its time |
| The service down, or slow | The same, and tried again later |
| No town chosen yet | Not activated: no fetching, no file; the apps and the widget say weather isn't set up |
| Weather switched off | No fetching; the apps say weather is off |
| `zoomriya-weather` stopped | The apps read the last file; nothing else on the box depends on it: sign-in, photos, messages and backups never wait for the weather |
| ZOOMRIYA Connect or our servers down | No effect: the box fetches directly from the weather service |

## 9. Open decisions

| # | Question | Options | State |
| --- | --- | --- | --- |
| W-1 | The list of towns shipped with the OS | GeoNames `cities1000` (every place over 1,000 people, the same list the apps use for place names), with `admin1CodesASCII` for each town's province or state, cut down to the name, province or state, country, coordinates and population: a few megabytes in the image. Searched within the chosen country, bigger towns first. Never the street map (the apps', downloaded after setup, Canada and the US only, ODbL) and never the box's internet address (often the wrong town). Credit: "Place names from GeoNames (CC BY 4.0), shortened." under the list and on the widget ([Licences](licences.md)) | Decided (16 November 2026); built: 171,939 towns, 4 MB |
| W-2 | A box with no apps | A Box Desktop widget: the town, now, today and tomorrow, any warning | Decided (16 November 2026) |
| W-3 | More than one town | A family with a cottage or a child at university. One town now | Open |
| W-4 | Alerts outside Canada and the US | Built 16 November 2026: Norway from MET Norway's MetAlerts, and MeteoAlarm's 37 other European countries (§2). The rest of the world: not available yet ("Weather alerts aren't available for your country yet."). Next: the IFRC Alert Hub once IFRC confirms its terms; or a country's own CAP feed from the WMO Register of Alerting Authorities, each checked for commercial use first. GDACS isn't used (§2). Proposed first, 16 November 2026: **MeteoAlarm** for Europe (EUMETNET, 38 national weather services' warnings, one RSS feed per country, today's and tomorrow's read separately; its terms allow redistribution, commercial included, on CC BY 4.0's terms, with the credit "EUMETNET – MeteoAlarm" and a link to meteoalarm.org), and the **IFRC Alert Hub** for the rest of the world (official alerting authorities' CAP alerts, GraphQL at `alerthub-api.ifrc.org`, its `Alerts` query; says it is for rebroadcasters such as weather apps, but publishes no data licence: ask IFRC before we ship it). Both in CAP, read the same way as ECCC's and the NWS's, the town matched against each alert's area on the box; a hub's alerts for Canada and the US left out, and duplicates removed by CAP `identifier` and `sender`. Backups: each country's own CAP feed from the WMO Register of Alerting Authorities | Norway and Europe built (16 November 2026); the rest of the world waits on the IFRC's terms |
| W-5 | Celsius or Fahrenheit | Fahrenheit in the US; Celsius everywhere else | Decided (16 November 2026) |

## 10. Plan

1. `zoomriya-weather` with ECCC, its account, the declared connection and the
   switch in Security › Internet Access. Built 16 November 2026.
2. The town in the first setup and in Control Panel. Built 16 November 2026.
3. NWS, then MET Norway. Built 16 November 2026.
4. The Box Desktop widget. Built 16 November 2026.
5. The apps: Myira's answers, Family Hub's card and the notices to every
   member. Built 16 November 2026; the phone app's notifications come with
   the phone app.
6. Alerts outside Canada and the US (W-4): Norway and MeteoAlarm's
   countries built 16 November 2026; then the IFRC Alert Hub once its terms
   allow it.

## 11. Built

| Piece | Where |
| --- | --- |
| The service, `zoomriya-weatherd` (Go, the standard library only): the town and the switches read each minute, the forecast every hour or at the answer's `Expires`, alerts every 10 minutes, a longer wait after each failure, the file written whole then moved into place | `packages/zoomriya-weather/src/` (`service.go`, `eccc.go`, `nws.go`, `metno.go`, tests on the services' real answers in `testdata/`) |
| Its account, unit, declared connection (`weather`, optional) | `packages/zoomriya-weather/usr/lib/sysusers.d/`, `usr/lib/systemd/system/zoomriya-weather.service`, `etc/zoomriya/outbound.d/weather.json` |
| The towns: GeoNames `cities1000` cut down to the id, name, province or state, country, coordinates, population and time zone, 171,939 towns in 4 MB, built into the package; and each European town's MeteoAlarm area codes (56,479 towns), worked out at build time from MeteoAlarm's newest published areas (`MeteoAlarm_Geocodes_<date>.json`); only the codes are shipped | `packages/zoomriya-weather/build` → `/usr/share/zoomriya/towns.tsv.gz` |
| Alerts in Norway and Europe | `packages/zoomriya-weather/src/europe.go` (MetAlerts, MeteoAlarm) |
| Family Hub's card and the notices: a warning starting and ending, kept for every member until they press Got it, never for each update of the same warning | zoomriya-apps: the box service's `app/weather.py` (`GET /box/api/v1/weather`, `POST /box/api/v1/weather/read`), Family Hub's `WeatherAlerts.svelte` |
| `town-set`, and the town in `setup-status`; another country forgets the town | The agent's `town.go` |
| The town search, the Town step, Control Panel's Town row, the widget | Box Desktop's `towns.go` (`GET /api/places/towns`, `GET /api/weather`), `Setup.svelte`, `Language.svelte`, `WeatherWidget.svelte` |
| Reading the file in the apps | zoomriya-apps: `zoomriya_common.box_weather`; Myira's `app/answers/weather.py` |

`weather.json`'s fields: `version` (1), `state` (`ok`, `stale`, `waiting`,
`off`), `town`, `service` (`eccc`, `nws`, `metno`), `credit` (English and
French), `licence`, `units` (`C` or `F`), `fetched`, `expires`,
`alerts_available`, `alerts_fetched`, `now` (time, temperature, sky, icon,
station), `days` (date, name, high, low, sky, icon, chance of
precipitation, the day's and the night's words), `alerts` (id, kind:
warning, watch, advisory or statement, event, headline, text, instruction,
area, issued, starts, ends), `official` and `official_name` (where the
service's own warnings are), and `problem` (why the last fetch failed).
Every text is in English and French; the NWS writes English only, so both
hold its English. No file: no town chosen.

## 12. Sources

| Service | Documentation |
| --- | --- |
| ECCC open data | <https://eccc-msc.github.io/open-data/readme_en/> |
| ECCC licence | <https://eccc-msc.github.io/open-data/licence/readme_en> |
| ECCC city page weather | <https://eccc-msc.github.io/open-data/msc-data/citypage-weather/readme_citypageweather_en> |
| ECCC weather alerts | <https://api.weather.gc.ca/collections/weather-alerts> |
| NWS API | <https://www.weather.gov/documentation/services-web-api> |
| MET Norway location forecast | <https://api.met.no/weatherapi/locationforecast/> |
| MET Norway MetAlerts | <https://docs.api.met.no/doc/metalerts/geojson>, licensing: <https://www.met.no/en/free-meteorological-data/Licensing-and-crediting> |
| MeteoAlarm feeds and terms | <https://feeds.meteoalarm.org/>, <https://www.meteoalarm.org/en/live/page/terms-and-conditions> |
| MeteoAlarm's areas (geocodes) | <https://gitlab.com/meteoalarm-pm-group/documents> |
| GDACS terms (not used) | <https://www.gdacs.org/documents/2025/GDACS_Terms_of_use_Mar_25.pdf> |
| MET Norway terms | <https://api.met.no/doc/TermsOfService> |
| CC BY 4.0 | <https://creativecommons.org/licenses/by/4.0/> |
