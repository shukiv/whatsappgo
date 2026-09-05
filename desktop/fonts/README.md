# Bundled fonts

Roboto is the interface face; every other file here is a script fallback, so
that no message draws as empty boxes on a machine without the matching system
font. All of them are Noto, under the SIL Open Font License 1.1 (Roboto is
Apache-2.0); see the COPYRIGHT files beside them.

`NotoSansThaana-Regular.ttf` is the one file that is not upstream's bytes. Qt
decides whether a font can render a complex script by looking for that script's
tag in GSUB alone, and no Noto Sans Thaana has a GSUB - Thaana needs mark
positioning, which lives in GPOS, and no substitutions at all. Qt therefore
refuses the face and Dhivehi text falls through to whatever the system has, or
to boxes. `tools/add-thaana-gsub.py` adds an empty GSUB naming the thaa and
DFLT scripts, which changes nothing about how the font draws and is enough for
Qt to accept it. The OFL permits the modification; Noto reserves no font name.

desktop-bundled-font covers the outcome: it asserts that a sample from each
bundled script is drawable by the interface font.
