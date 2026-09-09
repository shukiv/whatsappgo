# Lucide icons

Bundled from [Lucide 1.42.0](https://github.com/lucide-icons/lucide/tree/3859eb20fabe7fd95652fcd4395843b6c0bcdd01/icons),
commit `3859eb20fabe7fd95652fcd4395843b6c0bcdd01`. These SVGs are unmodified
upstream artwork, renamed to retain WhatsAppGo's semantic resource names.
See [LICENSE.lucide](LICENSE.lucide) for the ISC and Feather/MIT notices;
the license is also embedded in the application resources.

`TintedIcon` renders them via the `lucide` image provider at the target device's
pixel density, using the caller's theme tint. Qt's image cache shares repeated
icons; no web requests, JavaScript packages, SVG image-format plugin, or GPU
colorization effects are required. The selected `star-filled` variant fills the
same upstream star outline at render time. Add new icons to the CMake resource
list, keeping their 24-unit view box and 2-unit rounded stroke.

The app logo, status rings, and message artwork are not Lucide assets.

The two `PoweredBy_200px-…_HorizLogo.png` files are unmodified official GIPHY
attribution marks, not Lucide icons. Source: the archive linked from
[GIPHY API attribution](https://developers.giphy.com/docs/#attribution),
`https://media.giphy.com/giphy-attribution-marks.zip` (retrieved 2026-09-08).
Use the supplied light/dark artwork without recoloring it.

| App resource | Upstream icon |
| --- | --- |
| archive | archive |
| attach | paperclip |
| back | arrow-left |
| block | ban |
| bug | bug |
| calendar | calendar |
| camera | camera |
| calls | phone |
| channels | radio |
| chats | message-circle-more |
| check | check |
| check-check | check-check |
| chevron-down | chevron-down |
| chevron-right | chevron-right |
| close | x |
| communities | users-round |
| contact | contact-round |
| copy | copy |
| delete | trash |
| document | file-text |
| download | download |
| edit | pencil |
| forward | forward |
| gallery | image |
| group-add | users-round |
| headphones | headphones |
| heart | heart |
| info | info |
| link | link |
| lock | lock-keyhole |
| logout | log-out |
| menu | ellipsis-vertical |
| mic | mic |
| moon | moon |
| mute | bell-off |
| new-chat | message-circle-plus |
| pause | pause |
| phone | phone |
| pin | pin |
| play | play |
| plus | plus |
| poll | chart-no-axes-column-increasing |
| profile | user-round |
| reconnect | refresh-cw |
| reply | reply |
| rotate-left | rotate-ccw |
| rotate-right | rotate-cw |
| search | search |
| send | send |
| settings | settings |
| bell | bell |
| shield | shield-half |
| flag | flag |
| smile | face-slightly-smiling |
| sort | arrow-down-wide-narrow |
| star | star |
| star-filled | star |
| status | circle-dashed |
| sticker | sticker |
| stop | square |
| sun | sun |
| user | user-round |
| user-add | user-round-plus |
| video | video |

`view-once.svg` is a project-authored dashed-circle/1 status glyph, using the
same 24-unit canvas and 2-unit stroke as the Lucide controls.
