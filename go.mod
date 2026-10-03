module github.com/akrck02/go-simulator-urban

go 1.25.6

require github.com/akrck02/go-simulator v0.0.0

require github.com/twpayne/go-geos v0.23.0

require gopkg.in/yaml.v3 v3.0.1 // indirect

// developed next to these projects
replace (
	github.com/akrck02/go-renderer => ../go-renderer
	github.com/akrck02/go-simulator => ../go-simulator
)
