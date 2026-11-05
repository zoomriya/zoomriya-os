"""The first setup's words in the chosen language (gettext, domain
zoomriya-setup): English as written in the code, the others from the package's
po/ (scripts/i18n.py), compiled into /usr/share/locale. Box Desktop uses the
functions it shares from here (fixed_settings' messages) in its own language.

_("…") gives the words in the language in use; N_("…") only marks words kept
in a list, translated with _() where they're shown."""

from __future__ import annotations

import gettext
import os

DOMAIN = "zoomriya-setup"
#: The package's translations, or ZOOMRIYA_LOCALEDIR's (scripts/preview-*.sh build them).
LOCALEDIR = os.environ.get("ZOOMRIYA_LOCALEDIR", "/usr/share/locale")
#: The languages our screens come in, by gettext's code. Any other language
#: the box can have shows them in English.
OURS = ("en", "fr", "de", "es", "ta", "ja", "zh_CN")

_now: gettext.NullTranslations = gettext.NullTranslations()
_countries: gettext.NullTranslations = gettext.NullTranslations()
_subdivisions: gettext.NullTranslations = gettext.NullTranslations()
#: The language in use, one of OURS.
language = "en"


def ours(code: str) -> str:
    """A language as Debian names it (ta, zh, pt) as our screens show it:
    one of OURS, English for the rest."""
    code = code.split(".")[0]
    if code in OURS:
        return code
    base = code.split("_")[0]
    if base == "zh":
        return "zh_CN"
    return base if base in OURS else "en"


def use(code: str) -> None:
    """From now on, words in this language (a Debian language code); English
    where it has none."""
    global _now, _countries, _subdivisions, language
    language = ours(code)
    _now = gettext.translation(DOMAIN, LOCALEDIR, languages=[language], fallback=True)
    # Country names: Debian's iso-codes has them in nearly every language.
    _countries = gettext.translation("iso_3166-1", languages=[language], fallback=True)
    _subdivisions = gettext.translation("iso_3166-2", languages=[language], fallback=True)


def _(text: str) -> str:
    return _now.gettext(text)


def N_(text: str) -> str:
    return text


def country(name: str) -> str:
    """A country's English name in the language in use, where iso-codes has it."""
    return _countries.gettext(name)


def subdivision(name: str) -> str:
    """A state's or province's name in the language in use, where iso-codes has it."""
    return _subdivisions.gettext(name)
