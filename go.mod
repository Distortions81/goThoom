module go_client

go 1.24.3

require github.com/hajimehoshi/ebiten/v2 v2.8.8

require (
	github.com/Distortions81/EUI v0.0.31
	github.com/dustin/go-humanize v1.0.1
	github.com/hako/durafmt v0.0.0-20210608085754-5c1018a4e16b
	github.com/remeh/sizedwaitgroup v1.0.0
	github.com/sqweek/dialog v0.0.0-20240226140203-065105509627
	golang.org/x/crypto v0.41.0
	maze.io/x/math32 v0.0.0-20181106113604-c78ed91899f1
)

replace github.com/Distortions81/EUI => ../EUI/

require github.com/ebitengine/oto/v3 v3.3.3 // indirect

require (
	git.maze.io/go/math32 v0.0.0-20181106113604-c78ed91899f1 // indirect
	github.com/TheTitanrain/w32 v0.0.0-20200114052255-2654d97dbd3d // indirect
	github.com/ebitengine/gomobile v0.0.0-20250329061421-6d0a8e981e4c // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/go-text/typesetting v0.3.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	golang.org/x/image v0.30.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/time v0.12.0 // indirect
)
