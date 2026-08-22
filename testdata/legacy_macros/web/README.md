# Public legacy macro corpus

These `.mac` files are source text downloaded or extracted from public code
blocks on 2026-08-21 for compatibility testing. They are test fixtures, not
default-active macros. They are also embedded as opt-in library sources.

Each fixture has an inert `// Name: …` comment added by goThoom at its start.
It supplies the editable display name shown in the macro library; it does not
change legacy macro behavior.

| Fixture | Source | Attribution on source page |
| --- | --- | --- |
| `official-example.mac` | https://www.deltatao.com/clanlord/macros/example_macros.html | Delta Tao example contributors |
| `abbreviations.mac` | https://clump.clanlord.net/library/index.php?title=Abbreviations.mac | Noivad and many others |
| `keys.mac` | https://clump.clanlord.net/library/index.php?title=Keys.mac | Inu |
| `quickchain.mac` | https://clump.clanlord.net/library/index.php?title=Quickchain.mac | Noivad |
| `sunstone.mac` | https://clump.clanlord.net/library/index.php?title=Sunstone.mac | Noivad and Inu |
| `dances.mac` | https://clump.clanlord.net/library/index.php?title=Dances.mac | Inu; Mash by Noivad |
| `directions.mac` | https://clump.clanlord.net/library/index.php?title=Directions.mac | Kiriel D'Sol; adapted by Inu |
| `clump-scanner.mac` | https://clump.clanlord.net/library/index.php?title=Scanner.txt | Scanner.txt contributors |
| `clump-omega-zu.mac` | https://clump.clanlord.net/library/index.php?title=Omega_Zu | Omega Zu contributors |
| `gorvin-dynamicsharecads.mac` | http://gorvin.50webs.com/macros/dynamicsharecads.txt | Gorvin |
| `gorvin-right-clicker.mac` | http://gorvin.50webs.com/macros/RC2.txt | Gorvin |
| `gorvin-macro-chess.mac` | http://gorvin.50webs.com/macros/chessMac.txt | Gorvin |
| `gorvin-macro-tetris.mac` | http://gorvin.50webs.com/macros/tetrisMac.txt | Gorvin |

The test suite verifies the source can be parsed by goThoom's legacy macro
loader. Each source remains under its original author's copyright.
