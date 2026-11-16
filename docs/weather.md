# Weather

The box's weather: the forecast and the weather alerts for the family's town,
fetched by the box from a government weather service and kept on the box for
every app to read. Moved here from the apps' documents on 16 November 2026 and
brought up to date (first written 1 November 2026).

Not built yet. Everything below is the plan.

## 1. Decisions

| Decision | State |
| --- | --- |
| Weather is ZOOMRIYA OS's, box-wide like the time zone: one service of the OS fetches it, and every app reads what it kept (16 November 2026) | Decided |
| The weather service needs nothing from the apps: it works on a box with no apps, never calls one, reads none of their data and joins none of their groups. The apps get the weather from the box, and each decides what to show (16 November 2026) | Decided |
| No paid weather service: each country's government weather service, or MET Norway elsewhere, all free with commercial use allowed | Decided (1 November 2026) |
| Only the box talks to the weather services. Phones, laptops and browsers never do, and the apps never do | Decided (1 November 2026) |
| The family's town is chosen by the person installing, in the installer or the first setup, after the country, and must be filled in before Next. An admin changes it in Control Panel later | Decided (16 November 2026) |
| The OS image is built with no town: nothing is chosen at build time, and weather stays **not activated** (`zoomriya-weather` fetches nothing) until a town is chosen | Decided (16 November 2026) |
| Once the town is chosen, weather is on, and the same step says so: what leaves the box (the town, never anyone's name) and to whom. It has its own switch in Security › Internet Access | Decided (16 November 2026) |
| Alerts are for Canada and the US only. Elsewhere the forecast comes without alerts, and the box says "Weather alerts aren't available for your country yet." until we add that country's own alerts service | Decided (16 November 2026) |
| A Box Desktop widget shows the town's weather, so a box with no apps shows it too | Decided (16 November 2026) |
| Temperatures in Fahrenheit for a box in the US, in Celsius everywhere else (by the box's region) | Decided (16 November 2026) |
| Alerts reach the family three ways: Myira's answers, a notification to every member when a warning starts, and a card on Family Hub's dashboard while it lasts | Decided (16 November 2026); the three are the apps', each from what the box kept |
| Never a member's own location: not their phone's, not Family Tracker's | Decided (1 November 2026) |
| Not an emergency service: the box passes on what the weather service said, and says where official warnings are | Decided (1 November 2026) |

## 2. Where the weather comes from

| The family's country | Service | Forecast | Alerts |
| --- | --- | --- | --- |
| Canada | Environment and Climate Change Canada (ECCC), its MSC GeoMet service (`api.weather.gc.ca`): the city page forecasts, and the current weather alerts layer (it replaced the experimental one in GeoMet 3.0, February 2026) | Yes | Yes |
| United States | The National Weather Service (NWS, `api.weather.gov`): the forecast for a point, and active alerts | Yes | Yes |
| Anywhere else | MET Norway (`api.met.no`), its location forecast | Yes, hourly, worldwide | Not available until we add that country's own alerts service (MET Norway's alerts cover Norway only) |

When the service doesn't answer, the box keeps the last forecast, marked with
when it was fetched (§5). It never switches to another service by itself.

## 3. Licences

Checked 16 November 2026; checked again before each release.

| Service | Licence | Commercial use | Credit shown with the weather |
| --- | --- | --- | --- |
| ECCC | ECCC's Data Servers End-use Licence, version 2.1 (September 2022), with its Service Usage Policy | Allowed, free, no account | "Contains information licenced under the Data Server End-use Licence of Environment and Climate Change Canada." |
| NWS | A US government work: public domain | Allowed, free, no account | "From the National Weather Service." Each request names ZOOMRIYA in its `User-Agent`, as NWS asks |
| MET Norway | Its terms of service; the data under CC BY 4.0 | Allowed, free, no account | "Weather data from MET Norway, CC BY 4.0." Each request names ZOOMRIYA in its `User-Agent`, and the box keeps to each answer's `Expires`, as its terms ask |

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
| The town | Chosen by the person installing, in the installer or the first setup, from GeoNames' towns shipped with the OS (each with its coordinates, §9 W-1). Kept in `/etc/zoomriya/town` beside the region. The image has no `/etc/zoomriya/town`: until it exists, the service starts, finds no town and fetches nothing |

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
| The town: ECCC's site code for it, or its coordinates rounded to 0.01° (about 1 km) for NWS and MET Norway | Anyone's name, account, phone or device |
| The box's internet address, as with any connection | A member's location, from their phone or Family Tracker |
| `User-Agent: ZOOMRIYA-OS/<version> (https://zoomriya.com)` | The family's photos, messages, or anything else |

The service logs only whether a fetch worked, how long it took and the
service's answer code: never the town or its coordinates.

## 7. What the apps show

| Where | What |
| --- | --- |
| Box Desktop (the OS's own) | A widget: the town, now, today and tomorrow, and any active warning, with the credit and when it was fetched. On a box with no apps too |
| Myira | "What's the weather tomorrow?", "Is there a warning?": the forecast and alerts as typed blocks, with the credit and when it was fetched. An active warning comes first in any weather answer. With weather off: what Myira says today, the usual weather from the climate normals |
| Notifications | Every member, when a warning starts for the town, and when it ends. Not for every update of the same warning |
| Family Hub | A card at the top of the dashboard while a warning lasts: what it is, until when, and the credit |
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
| W-1 | The list of towns shipped with the OS | GeoNames `cities1000` (every place over 1,000 people, the same list the apps use for place names), with `admin1CodesASCII` for each town's province or state, cut down to the name, province or state, country, coordinates and population: a few megabytes in the image. Searched within the chosen country, bigger towns first. Never the street map (the apps', downloaded after setup, Canada and the US only, ODbL) and never the box's internet address (often the wrong town). Credit: "Place names from GeoNames (CC BY 4.0)." ([Licences](licences.md)) | Decided (16 November 2026) |
| W-2 | A box with no apps | A Box Desktop widget: the town, now, today and tomorrow, any warning | Decided (16 November 2026) |
| W-3 | More than one town | A family with a cottage or a child at university. One town now | Open |
| W-4 | Alerts outside Canada and the US | Not available ("Weather alerts aren't available for your country yet.") until we add each country's own service | Decided (16 November 2026) |
| W-5 | Celsius or Fahrenheit | Fahrenheit in the US; Celsius everywhere else | Decided (16 November 2026) |

## 10. Plan

1. `zoomriya-weather` with ECCC, its account, the declared connection and the
   switch in Security › Internet Access.
2. The town in the installer and the first setup, and in Control Panel.
3. NWS, then MET Norway.
4. The Box Desktop widget.
5. The apps: Myira's answers, the notifications, Family Hub's card.

## 11. Sources

| Service | Documentation |
| --- | --- |
| ECCC open data | <https://eccc-msc.github.io/open-data/readme_en/> |
| ECCC licence | <https://eccc-msc.github.io/open-data/licence/readme_en> |
| ECCC city page weather | <https://eccc-msc.github.io/open-data/msc-data/citypage-weather/readme_citypageweather_en> |
| ECCC weather alerts | <https://api.weather.gc.ca/collections/weather-alerts> |
| NWS API | <https://www.weather.gov/documentation/services-web-api> |
| MET Norway location forecast | <https://api.met.no/weatherapi/locationforecast/> |
| MET Norway terms | <https://api.met.no/doc/TermsOfService> |
| CC BY 4.0 | <https://creativecommons.org/licenses/by/4.0/> |
