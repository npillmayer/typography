module github.com/npillmayer/typography

go 1.25.6

require (
	github.com/npillmayer/cords v0.1.2-alpha.3
	github.com/npillmayer/khipu v0.0.0
	github.com/npillmayer/opentype v0.0.0
	github.com/npillmayer/uax v0.3.0-alpha1
	github.com/rivo/uniseg v0.4.7
	golang.org/x/image v0.34.0
	golang.org/x/text v0.32.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jolestar/go-commons-pool v2.0.0+incompatible // indirect
	github.com/npillmayer/schuko v0.2.0-alpha.3.0.20211209143531-2d524c4964ff // indirect
)

replace github.com/npillmayer/cords => ../cords

replace github.com/npillmayer/khipu => ../khipu

replace github.com/npillmayer/uax => ../uax

replace github.com/npillmayer/schuko => ../schuko

replace github.com/npillmayer/opentype => ../opentype
