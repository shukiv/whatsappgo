#!/usr/bin/env python3
"""Add an empty GSUB table to a font so Qt will use it for a complex script.

Qt decides whether a font can render a complex script by looking for the
script's tag in GSUB alone (QFontEngine::supportsScript), and Noto Sans Thaana
ships GPOS only - Thaana needs mark positioning, not substitutions. The result
is that Qt refuses the bundled face and Dhivehi text draws as empty boxes on
any machine without a system Thaana font.

An empty GSUB with the thaa and DFLT scripts and no lookups changes nothing
about how the font draws, and is enough for Qt to accept it.

Usage: add-thaana-gsub.py <font.ttf> [script tag ...]
Requires fonttools (pip install fonttools).
"""

import sys

from fontTools.ttLib import TTFont, newTable
from fontTools.ttLib.tables import otTables


def empty_gsub(script_tags):
    gsub = otTables.GSUB()
    gsub.Version = 0x00010000

    script_list = otTables.ScriptList()
    script_list.ScriptRecord = []
    for tag in script_tags:
        record = otTables.ScriptRecord()
        record.ScriptTag = tag
        script = otTables.Script()
        language = otTables.LangSys()
        language.LookupOrder = None
        language.ReqFeatureIndex = 0xFFFF
        language.FeatureIndex = []
        language.FeatureCount = 0
        script.DefaultLangSys = language
        script.LangSysRecord = []
        script.LangSysCount = 0
        record.Script = script
        script_list.ScriptRecord.append(record)
    script_list.ScriptCount = len(script_list.ScriptRecord)

    features = otTables.FeatureList()
    features.FeatureRecord = []
    features.FeatureCount = 0
    lookups = otTables.LookupList()
    lookups.Lookup = []
    lookups.LookupCount = 0

    gsub.ScriptList = script_list
    gsub.FeatureList = features
    gsub.LookupList = lookups
    return gsub


def main(argv):
    if len(argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2
    path = argv[1]
    tags = argv[2:] or ["DFLT", "thaa"]
    font = TTFont(path)
    if "GSUB" in font:
        print(f"{path} already has a GSUB table; nothing to do", file=sys.stderr)
        return 1
    table = newTable("GSUB")
    table.table = empty_gsub(tags)
    font["GSUB"] = table
    font.save(path)
    print(f"added an empty GSUB ({', '.join(tags)}) to {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
