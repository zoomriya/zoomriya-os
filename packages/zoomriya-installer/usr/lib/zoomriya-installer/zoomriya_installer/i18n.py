"""The installer's words in the chosen language (gettext, domain
zoomriya-installer): English as written in the code, the others from the
package's po/ (scripts/i18n.py), compiled into /usr/share/locale.

_("…") gives the words in the language in use; N_("…") only marks words kept
in a list, translated with _() where they're shown."""

from __future__ import annotations

import gettext
import os

DOMAIN = "zoomriya-installer"
#: The package's translations, or ZOOMRIYA_LOCALEDIR's (scripts/preview-*.sh build them).
LOCALEDIR = os.environ.get("ZOOMRIYA_LOCALEDIR", "/usr/share/locale")
#: Every language our screens are translated into (po/), by gettext's code.
TRANSLATED = ["en", "fr", "de", "es", "ta", "ja", "zh_CN"]
#: The languages offered, each named in itself (box-desktop.md §5, decided
#: 13 November 2026): English and French, until a native speaker has checked
#: another. The others stay translated and hidden; switching one on is adding
#: it here (and to Box Desktop's lists). server.py refuses the rest.
LANGUAGES = [("en", "English"), ("fr", "Français")]

_now: gettext.NullTranslations = gettext.NullTranslations()
#: The language in use.
language = "en"


def use(code: str) -> None:
    """From now on, words in this language; English for a hidden one, or where it has none."""
    global _now, language
    if code not in dict(LANGUAGES):
        code = "en"
    language = code
    _now = gettext.translation(DOMAIN, LOCALEDIR, languages=[code], fallback=True)


def _(text: str) -> str:
    return _now.gettext(text)


def N_(text: str) -> str:
    return text
