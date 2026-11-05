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
#: The languages our screens come in, by gettext's code, each named in itself.
LANGUAGES = [("en", "English"), ("fr", "Français"), ("de", "Deutsch"), ("es", "Español"),
             ("ta", "தமிழ்"), ("ja", "日本語"), ("zh_CN", "简体中文")]

_now: gettext.NullTranslations = gettext.NullTranslations()
#: The language in use.
language = "en"


def use(code: str) -> None:
    """From now on, words in this language; English where it has none."""
    global _now, language
    language = code
    _now = gettext.translation(DOMAIN, LOCALEDIR, languages=[code], fallback=True)


def _(text: str) -> str:
    return _now.gettext(text)


def N_(text: str) -> str:
    return text
