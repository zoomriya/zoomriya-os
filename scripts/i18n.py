#!/usr/bin/python3 -I
"""Our screens' translations (gettext), with Python alone: no gettext tools
needed to build.

    scripts/i18n.py extract PACKAGE   # packages/PACKAGE/po/<lang>.po, from its _("…") and N_("…")
    scripts/i18n.py compile PO MO     # one translation, as gettext reads it
    scripts/i18n.py check PACKAGE     # what each language still lacks
    scripts/i18n.py locales DIR       # every package's, compiled under DIR, for the previews
    scripts/i18n.py json OUT PO...    # one language's translations for Box Desktop's page, as JSON

Extract keeps every translation already made, adds the new words untranslated,
and drops words the code no longer has. A translation still to be checked by a
native speaker is marked "fuzzy", as gettext does, and is used all the same.
"""

from __future__ import annotations

import ast
import json
import re
import struct
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
#: The languages our screens are translated into (gettext's codes).
LANGUAGES = ["fr", "de", "es", "ta", "ja", "zh_CN"]
#: Where each package's Python is.
SOURCES = {
    "zoomriya-installer": "usr/lib/zoomriya-installer/zoomriya_installer",
}
#: Where each web page's TypeScript and Svelte are: _("…") and N_("…") with
#: one plain string, as in the Python.
WEB_SOURCES = {
    "box-desktop": "web/src",
    # The installer's page lives with Box Desktop's (it shares its parts);
    # its words are the installer's, with its Python's (the drives, the install).
    "zoomriya-installer": "../box-desktop/web/src/installer",
}
_JS_CALL = re.compile(r"""(?<![\w$.])N?_\(\s*('(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*")\s*\)""")
_JS_ESCAPES = {"n": "\n", "t": "\t", "\\": "\\", "'": "'", '"': '"'}


def js_string(literal: str) -> str:
    """A JavaScript string literal's text."""
    return re.sub(r"\\(.)", lambda m: _JS_ESCAPES.get(m.group(1), m.group(1)), literal[1:-1])


def found_web_words(package: str) -> dict[str, list[str]]:
    words: dict[str, list[str]] = {}
    source = ROOT / "packages" / package / WEB_SOURCES[package]
    for path in sorted(list(source.rglob("*.ts")) + list(source.rglob("*.svelte"))):
        if path.name.endswith(".test.ts"):
            continue
        # Another package's page in the same folder (the installer's in Box Desktop's).
        nested = [(ROOT / "packages" / other / where).resolve() for other, where in WEB_SOURCES.items()
                  if other != package]
        if any(inner != source.resolve() and inner.is_relative_to(source.resolve())
               and path.resolve().is_relative_to(inner) for inner in nested):
            continue
        text = path.read_text()
        for match in _JS_CALL.finditer(text):
            start = text.rfind("\n", 0, match.start()) + 1
            if text[start:match.start()].lstrip().startswith(("//", "*", "/*")):
                continue  # a comment that names _("…"), as the Python's docstrings do
            line = text.count("\n", 0, match.start()) + 1
            rel = path.relative_to(source.resolve()) if path.is_relative_to(source.resolve()) else path.relative_to(source)
            words.setdefault(js_string(match.group(1)), []).append(f"{rel}:{line}")
    return words


def found_words(package: str) -> dict[str, list[str]]:
    """Every _("…") and N_("…") with a plain string, and where it is."""
    words: dict[str, list[str]] = {}
    if package in WEB_SOURCES:
        words = found_web_words(package)
        if package not in SOURCES:
            return words
    source = ROOT / "packages" / package / SOURCES[package]
    for path in sorted(source.glob("*.py")):
        tree = ast.parse(path.read_text(), str(path))
        for node in ast.walk(tree):
            if (isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
                    and node.func.id in ("_", "N_") and len(node.args) == 1
                    and isinstance(node.args[0], ast.Constant) and isinstance(node.args[0].value, str)):
                words.setdefault(node.args[0].value, []).append(f"{path.name}:{node.lineno}")
    return words


def quote(text: str) -> str:
    return '"' + text.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n").replace("\t", "\\t") + '"'


def unquote(text: str) -> str:
    return ast.literal_eval(text)


def read_po(path: Path) -> dict[str, tuple[str, bool]]:
    """A .po file's translations: {original: (translation, fuzzy)}."""
    found: dict[str, tuple[str, bool]] = {}
    if not path.exists():
        return found
    fuzzy, field, parts = False, "", {"msgid": "", "msgstr": ""}

    def keep() -> None:
        if parts["msgid"]:
            found[parts["msgid"]] = (parts["msgstr"], fuzzy)

    for line in path.read_text().splitlines() + [""]:
        line = line.strip()
        if not line:
            keep()
            fuzzy, field, parts = False, "", {"msgid": "", "msgstr": ""}
        elif line.startswith("#,") and "fuzzy" in line:
            fuzzy = True
        elif line.startswith("#"):
            continue
        elif line.startswith(("msgid ", "msgstr ")):
            field, _, rest = line.partition(" ")
            parts[field] = unquote(rest)
        elif line.startswith('"') and field:
            parts[field] += unquote(line)
    return found


def write_po(path: Path, package: str, lang: str, words: dict[str, list[str]],
             done: dict[str, tuple[str, bool]]) -> None:
    lines = [f"# {package}: {lang}. Made by scripts/i18n.py extract; translate the empty msgstr.",
             "# \"fuzzy\": a draft still to be checked by a native speaker.",
             'msgid ""',
             'msgstr ""',
             quote("Content-Type: text/plain; charset=UTF-8\n"),
             quote(f"Language: {lang}\n"),
             ""]
    for original, where in words.items():
        translation, fuzzy = done.get(original, ("", False))
        lines.append("#: " + " ".join(where))
        if fuzzy:
            lines.append("#, fuzzy")
        lines.append("msgid " + quote(original))
        lines.append("msgstr " + quote(translation))
        lines.append("")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines))


def extract(package: str) -> None:
    words = found_words(package)
    for lang in LANGUAGES:
        path = ROOT / "packages" / package / "po" / f"{lang}.po"
        write_po(path, package, lang, words, read_po(path))
    print(f"{package}: {len(words)} texts")


def compile_po(po: Path, mo: Path) -> None:
    """GNU's .mo: a header, the originals' and translations' tables, then the texts."""
    entries = {k: v for k, (v, _f) in read_po(po).items() if v}
    entries[""] = "Content-Type: text/plain; charset=UTF-8\n"
    keys = sorted(entries)
    ids = [k.encode() for k in keys]
    strs = [entries[k].encode() for k in keys]
    start = 7 * 4 + 16 * len(keys)
    offsets, blob = [], b""
    for text in ids + strs:
        offsets.append((len(text), start + len(blob)))
        blob += text + b"\0"
    head = struct.pack("Iiiiiii", 0x950412DE, 0, len(keys), 7 * 4, 7 * 4 + 8 * len(keys), 0, 0)
    tables = b"".join(struct.pack("ii", *o) for o in offsets[:len(keys)])
    tables += b"".join(struct.pack("ii", *o) for o in offsets[len(keys):])
    mo.parent.mkdir(parents=True, exist_ok=True)
    mo.write_bytes(head + tables + blob)


def write_json(out: Path, pos: list[Path]) -> None:
    """The web Box Desktop's words in one language: {English: translation},
    from these .po files, the later ones winning. Like compile, drafts are
    used and untranslated words left out, so the page shows them in English."""
    words: dict[str, str] = {}
    for po in pos:
        words.update({k: v for k, (v, _f) in read_po(po).items() if k and v})
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(dict(sorted(words.items())), ensure_ascii=False, separators=(",", ":")) + "\n")


def check(package: str) -> None:
    words = found_words(package)
    for lang in LANGUAGES:
        done = read_po(ROOT / "packages" / package / "po" / f"{lang}.po")
        missing = [w for w in words if not done.get(w, ("", False))[0]]
        drafts = sum(1 for w in words if done.get(w, ("", False))[1])
        print(f"{lang}: {len(words) - len(missing)}/{len(words)} translated, {drafts} drafts to check")


def main() -> int:
    match sys.argv[1:]:
        case ["extract", package]:
            extract(package)
        case ["compile", po, mo]:
            compile_po(Path(po), Path(mo))
        case ["json", out, *pos] if pos:
            write_json(Path(out), [Path(p) for p in pos])
        case ["check", package]:
            check(package)
        case ["locales", out]:
            for package in SOURCES:
                for po in sorted((ROOT / "packages" / package / "po").glob("*.po")):
                    compile_po(po, Path(out) / po.stem / "LC_MESSAGES" / f"{package}.mo")
        case _:
            print(__doc__, file=sys.stderr)
            return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
