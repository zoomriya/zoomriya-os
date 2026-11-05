"""Box Desktop's words in the box's language (gettext, domain
box-desktop-app): English as written in the code, the others from the
package's po/ (scripts/i18n.py), compiled into /usr/share/locale. The words it
shows from the first setup's code (fixed_settings' messages, the countries)
follow the same language.

_("…") gives the words in the language in use; N_("…") only marks words kept
in a list, translated with _() where they're shown."""

from __future__ import annotations

import gettext
import locale

from zoomriya_setup import i18n as setup_i18n

DOMAIN = "box-desktop-app"
LOCALEDIR = setup_i18n.LOCALEDIR

_now: gettext.NullTranslations = gettext.NullTranslations()


def language() -> str:
    """The language in use, one of zoomriya_setup.i18n.OURS."""
    return setup_i18n.language


def use(code: str) -> None:
    """From now on, words in this language (a Debian locale or language code,
    such as fr_CA.UTF-8 or ta); English where it has none."""
    global _now
    setup_i18n.use(code)
    _now = gettext.translation(DOMAIN, LOCALEDIR, languages=[setup_i18n.language], fallback=True)
    # Dates in it too (day and month names), once the box has built that
    # locale (the agent's language-set); until then, as before.
    if "_" in code:
        name = code if "." in code else code + ".UTF-8"
        try:
            locale.setlocale(locale.LC_TIME, name)
        except locale.Error:
            pass


def _(text: str) -> str:
    return _now.gettext(text)


def N_(text: str) -> str:
    return text
