package urban

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/akrck02/go-simulator/world"
)

// Terrain is the relief the city grows on: heights in metres on a grid over the ground plane.
// Rows go from MinimumZ, columns from MinimumX; values sit at cell centres.
type Terrain struct {
	Columns, Rows      int
	MinimumX, MinimumZ float64
	CellSize           float64 // world units (km) between cell centres
	heights            []float64
	slopeX, slopeZ     []float64 // metres per metre
}

// TerrainFile names a relief grid on disk: a NumPy .npy file of heights in metres, rows from the
// minimum z, and where its first cell centre is.
type TerrainFile struct {
	Path     string  `json:"path"`
	MinimumX float64 `json:"minimum_x"` // x of the first column's cell centres
	MinimumZ float64 `json:"minimum_z"` // z of the first row's cell centres
	CellSize float64 `json:"cell_size"`
}

// LoadTerrain reads a relief grid.
func LoadTerrain(file TerrainFile) (*Terrain, error) {
	heights, rows, columns, err := readNumpy(file.Path)
	if err != nil {
		return nil, err
	}
	if file.CellSize <= 0 {
		return nil, fmt.Errorf("relief %s: cell_size must be positive", file.Path)
	}
	terrain := &Terrain{Columns: columns, Rows: rows, MinimumX: file.MinimumX, MinimumZ: file.MinimumZ, CellSize: file.CellSize, heights: heights}
	terrain.computeSlopes()
	return terrain, nil
}

// TerrainFromField takes the ground from a world field of heights in metres (the elevation the
// terrain plugin publishes): rows from the smallest z, values at cell centres.
func TerrainFromField(field *world.Field) (*Terrain, error) {
	if field.Columns < 2 || field.Rows < 2 {
		return nil, fmt.Errorf("the elevation field needs at least 2 × 2 cells")
	}
	cellSize := (field.Extent.MaximumX - field.Extent.MinimumX) / float64(field.Columns)
	heights := make([]float64, len(field.Values))
	for index, value := range field.Values {
		heights[index] = float64(value)
	}
	terrain := &Terrain{Columns: field.Columns, Rows: field.Rows, CellSize: cellSize, heights: heights,
		MinimumX: field.Extent.MinimumX + cellSize/2, MinimumZ: field.Extent.MinimumZ + cellSize/2}
	terrain.computeSlopes()
	return terrain, nil
}

// computeSlopes takes the gradient like numpy.gradient: central differences inside, one-sided at
// the edges, in metres per metre.
func (terrain *Terrain) computeSlopes() {
	metres := terrain.CellSize * 1000
	terrain.slopeX = make([]float64, len(terrain.heights))
	terrain.slopeZ = make([]float64, len(terrain.heights))
	for row := range terrain.Rows {
		for column := range terrain.Columns {
			index := row*terrain.Columns + column
			terrain.slopeX[index] = difference(terrain.heights, index, column, terrain.Columns, 1) / metres
			terrain.slopeZ[index] = difference(terrain.heights, index, row, terrain.Rows, terrain.Columns) / metres
		}
	}
}

func difference(values []float64, index, position, count, stride int) float64 {
	switch {
	case count < 2:
		return 0
	case position == 0:
		return values[index+stride] - values[index]
	case position == count-1:
		return values[index] - values[index-stride]
	}
	return (values[index+stride] - values[index-stride]) / 2
}

// sample interpolates a grid bilinearly at a point, clamping to the edges.
func (terrain *Terrain) sample(values []float64, x, z float64) float64 {
	column := clamp((x-terrain.MinimumX)/terrain.CellSize, 0, float64(terrain.Columns-1))
	row := clamp((z-terrain.MinimumZ)/terrain.CellSize, 0, float64(terrain.Rows-1))
	left, top := int(math.Floor(column)), int(math.Floor(row))
	right, bottom := min(left+1, terrain.Columns-1), min(top+1, terrain.Rows-1)
	across, down := column-float64(left), row-float64(top)
	at := func(row, column int) float64 { return values[row*terrain.Columns+column] }
	upper := at(top, left)*(1-across) + at(top, right)*across
	lower := at(bottom, left)*(1-across) + at(bottom, right)*across
	return upper*(1-down) + lower*down
}

// Height returns the height in metres at a point.
func (terrain *Terrain) Height(x, z float64) float64 { return terrain.sample(terrain.heights, x, z) }

// Slope returns the steepness at a point (metres per metre).
func (terrain *Terrain) Slope(x, z float64) float64 {
	return math.Hypot(terrain.sample(terrain.slopeX, x, z), terrain.sample(terrain.slopeZ, x, z))
}

// ContourBearing returns the direction of the contour line at a point (radians in x, z).
func (terrain *Terrain) ContourBearing(x, z float64) float64 {
	return math.Atan2(terrain.sample(terrain.slopeZ, x, z), terrain.sample(terrain.slopeX, x, z)) + math.Pi/2
}

func clamp(value, lowest, highest float64) float64 { return math.Max(lowest, math.Min(highest, value)) }

// ---------------------------------------------------------------- NumPy files

var shapePattern = regexp.MustCompile(`'shape':\s*\((\d+),\s*(\d+)\s*\)`)

// readNumpy reads a two-dimensional .npy array of little-endian float32 or float64 in C order.
func readNumpy(path string) (values []float64, rows, columns int, err error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	header, data, err := splitNumpy(contents)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	if strings.Contains(header, "'fortran_order': True") {
		return nil, 0, 0, fmt.Errorf("%s: Fortran-ordered arrays are not supported", path)
	}
	match := shapePattern.FindStringSubmatch(header)
	if match == nil {
		return nil, 0, 0, fmt.Errorf("%s: expected a two-dimensional array", path)
	}
	rows, _ = strconv.Atoi(match[1])
	columns, _ = strconv.Atoi(match[2])
	values, err = decodeNumbers(header, data, rows*columns)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	return values, rows, columns, nil
}

func splitNumpy(contents []byte) (string, []byte, error) {
	if len(contents) < 10 || string(contents[1:6]) != "NUMPY" {
		return "", nil, fmt.Errorf("not a .npy file")
	}
	headerLength, start := int(binary.LittleEndian.Uint16(contents[8:10])), 10
	if contents[6] >= 2 {
		headerLength, start = int(binary.LittleEndian.Uint32(contents[8:12])), 12
	}
	if start+headerLength > len(contents) {
		return "", nil, fmt.Errorf("truncated header")
	}
	return string(contents[start : start+headerLength]), contents[start+headerLength:], nil
}

func decodeNumbers(header string, data []byte, count int) ([]float64, error) {
	values := make([]float64, count)
	switch {
	case strings.Contains(header, "'<f4'"):
		if len(data) < 4*count {
			return nil, fmt.Errorf("truncated data")
		}
		for index := range values {
			values[index] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[4*index:])))
		}
	case strings.Contains(header, "'<f8'"):
		if len(data) < 8*count {
			return nil, fmt.Errorf("truncated data")
		}
		for index := range values {
			values[index] = math.Float64frombits(binary.LittleEndian.Uint64(data[8*index:]))
		}
	default:
		return nil, fmt.Errorf("only little-endian float32 or float64 arrays are supported")
	}
	return values, nil
}

// terrainFileFrom reads the "relief" parameter of the city rule.
func terrainFileFrom(value any) (TerrainFile, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return TerrainFile{}, err
	}
	var file TerrainFile
	if err := json.Unmarshal(encoded, &file); err != nil || file.Path == "" {
		return TerrainFile{}, fmt.Errorf("relief needs {path, minimum_x, minimum_z, cell_size}")
	}
	return file, nil
}
