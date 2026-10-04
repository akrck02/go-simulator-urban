package urban

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"

	"github.com/akrck02/go-simulator/plugin"
	"github.com/twpayne/go-geos"
)

// Parcel uses.
const (
	UseHousing    = "housing"
	UseShop       = "shop"
	UseWorkshop   = "workshop"
	UseExtensive  = "extensive"  // gardens, convents: the belts where old walls stood
	UseDemolished = "demolished" // taken by a new street, square or wall
)

// Parcel is a burgage plot along a street: a house on the front and a garden behind.
type Parcel struct {
	Identifier        int
	Polygon           *geos.Geom
	Centre            Point
	Edge              int     // street it faces
	Front, Depth      float64 // km
	Direction, Normal Point   // along the street; from the street into the plot
	SquareMetres      float64
	Coverage          float64 // share of the plot the building covers
	BuildingDepth     float64 // km, when houses grow by depth from their front (see Scenario.BuildingDepth)
	BackDepth         float64 // km, the building at the back of the plot, round a yard (see DensitySettings)
	Floors            int
	Use               string
	Year              float64
	footprint         *geos.Geom
	footprintOf       footprintKey // what the cached footprint was built from
}

// footprintKey is what a building's footprint depends on.
type footprintKey struct {
	coverage float64
	use      string
	polygon  *geos.Geom
}

// CachedFootprint returns the building's footprint, building it again only when the plot changed.
func (parcel *Parcel) CachedFootprint() *geos.Geom {
	key := footprintKey{parcel.Coverage, parcel.Use, parcel.Polygon}
	if parcel.footprint == nil || parcel.footprintOf != key {
		parcel.footprint, parcel.footprintOf = parcel.Footprint(), key
	}
	return parcel.footprint
}

// BuiltSquareMetres is the floor space of the parcel.
func (parcel *Parcel) BuiltSquareMetres() float64 {
	return parcel.SquareMetres * parcel.Coverage * float64(parcel.Floors)
}

func (parcel *Parcel) standing() bool { return parcel.Use != UseDemolished }

// Wall is a city wall: its area, its line (without the stretch along the rock) and its gates.
type Wall struct {
	Polygon   *geos.Geom
	Line      *geos.Geom
	Gates     []Point
	Year      float64
	clearZone *geos.Geom
}

// clearance is the strip along the wall where nothing is built (computed once).
func (wall *Wall) clearance() *geos.Geom {
	if wall.clearZone == nil {
		wall.clearZone = wall.Line.Buffer(wallClearance, 8)
	}
	return wall.clearZone
}

// frontage is one side of a street piece.
type frontage struct {
	edge int
	side int // −1 or +1
}

// City is the state of one simulated city, shared by the rules that work on it.
type City struct {
	Name                                   string
	Scenario                               *Scenario
	Terrain                                *Terrain
	Window, Obstacles, Rivers, WallSupport *geos.Geom
	RiverAxes                              []*riverAxis
	ClearArea                              *geos.Geom // where the drawn scene's old content gives way to the simulated city
	WallOutline                            *geos.Geom // the shape the wall has when it goes up, when it is given (it is the only wall)
	Network                                *Network
	Parcels                                []*Parcel
	Walls                                  []*Wall
	Belts                                  *geos.Geom // where old walls stood
	Squares                                []*geos.Geom
	Bridges                                []Segment
	Year                                   float64
	Deficit                                float64 // square metres of floor space still missing this year

	random           *rand.Rand
	parcelCells      map[cellKey]map[int]bool
	frontagesDone    map[frontage]bool
	frontagePriority map[int]float64 // this year's, by edge
	nodeWeights      map[int]float64 // this year's, by node
	weightsYear      float64
	published        map[string]string // entity identifier → signature of what was sent
}

// newCity loads a scenario's inputs and lays out the roads that already exist.
func newCity(name string, scenario *Scenario) (*City, error) {
	terrain, err := LoadTerrain(TerrainFile{Path: plugin.Path(scenario.Relief.Path), MinimumX: scenario.Relief.MinimumX,
		MinimumZ: scenario.Relief.MinimumZ, CellSize: scenario.Relief.CellSize})
	if err != nil {
		return nil, err
	}
	city := &City{Name: name, Scenario: scenario, Terrain: terrain, Network: NewNetwork(), Belts: emptyGeometry(),
		Window: emptyGeometry(), Obstacles: emptyGeometry(), Rivers: emptyGeometry(), WallSupport: emptyGeometry(), ClearArea: emptyGeometry(), WallOutline: emptyGeometry(),
		random: rand.New(rand.NewPCG(scenario.Seed, 0x5eed)), parcelCells: map[cellKey]map[int]bool{},
		frontagesDone: map[frontage]bool{}, published: map[string]string{}, weightsYear: math.NaN()}
	if scenario.Features != "" {
		if err := city.loadFeatures(plugin.Path(scenario.Features)); err != nil {
			return nil, err
		}
	}
	if city.Window.IsEmpty() {
		return nil, fmt.Errorf("city %q: the features need a \"window\" polygon (where the city may grow)", name)
	}
	return city, nil
}

// loadFeatures reads the GeoJSON inputs: every feature has a "role".
func (city *City) loadFeatures(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var collection struct {
		Features []struct {
			Properties map[string]any  `json:"properties"`
			Geometry   json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(contents, &collection); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for index, feature := range collection.Features {
		geometry, err := geos.NewGeomFromGeoJSON(string(feature.Geometry))
		if err != nil {
			return fmt.Errorf("%s, feature %d: %w", path, index, err)
		}
		if feature.Properties["role"] == "river" {
			width, _ := feature.Properties["width"].(float64)
			city.addRiver(geometry, positiveOr(width, 0.02))
			continue
		}
		if err := city.useFeature(feature.Properties["role"], valid(geometry)); err != nil {
			return fmt.Errorf("%s, feature %d: %w", path, index, err)
		}
	}
	return nil
}

func (city *City) useFeature(role any, geometry *geos.Geom) error {
	switch role {
	case "window":
		city.Window = unionOf(city.Window, geometry)
	case "obstacles":
		city.Obstacles = unionOf(city.Obstacles, geometry)
	case "rivers":
		city.Rivers = unionOf(city.Rivers, geometry)
	case "wall_support":
		city.WallSupport = unionOf(city.WallSupport, geometry)
	case "clear":
		city.ClearArea = unionOf(city.ClearArea, geometry)
	case "wall_outline":
		city.WallOutline = unionOf(city.WallOutline, geometry)
	case "road":
		for _, line := range linesOf(geometry) {
			city.Network.AddPath(coordinatesOf(line), ClassRoad, math.Inf(-1), 0.04)
		}
	default:
		return fmt.Errorf("unknown role %v (known: window, obstacles, rivers, wall_support, road, river, clear, wall_outline)", role)
	}
	return nil
}

// Population is the number of inhabitants this year.
func (city *City) Population() float64 { return city.Scenario.PopulationIn(city.Year) }

// BuiltSquareMetres is the floor space of every parcel in use.
func (city *City) BuiltSquareMetres() float64 {
	total := 0.0
	for _, parcel := range city.Parcels {
		if parcel.Use != UseExtensive {
			total += parcel.BuiltSquareMetres()
		}
	}
	return total
}

// settledHectares is the ground of the parcels plus 40 % for streets and squares (a quick
// estimate, used every year to decide whether to densify or to spread).
func (city *City) settledHectares() float64 {
	total := 0.0
	for _, parcel := range city.Parcels {
		if parcel.standing() {
			total += parcel.Polygon.Area()
		}
	}
	return total * 1.4 * 100
}

// expectedHectares is the ground a European city of this population covered around 1300
// (Cesaretti et al. 2016).
func expectedHectares(population float64) float64 {
	return math.Exp(-2.125) * math.Pow(population, 0.714)
}

// CurrentWall returns the newest wall, or nil.
func (city *City) CurrentWall() *Wall {
	if len(city.Walls) == 0 {
		return nil
	}
	return city.Walls[len(city.Walls)-1]
}

// insideTheWall tells whether a point is inside the newest wall.
func (city *City) insideTheWall(point Point) bool {
	wall := city.CurrentWall()
	return wall != nil && wall.Polygon.Contains(pointGeometry(point))
}

// closeness is how strongly the growth centres pull at a point, within a radius.
func (city *City) closeness(point Point, radius float64, combine func(values []float64) float64) float64 {
	var values []float64
	for _, centre := range city.Scenario.CentresIn(city.Year) {
		distance := point.DistanceTo(centre.point)
		values = append(values, centre.weight*math.Exp(-math.Pow(distance/radius, 2)))
	}
	return combine(values)
}

func largest(values []float64) float64 {
	best := math.Inf(-1)
	for _, value := range values {
		best = math.Max(best, value)
	}
	return best
}

func total(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum
}

// growthRadius is how far from the centres the city builds before it opens new streets; it grows
// with the population (the settled area goes as N^0.714, its radius as N^0.357).
func (city *City) growthRadius() float64 {
	hectares := expectedHectares(math.Max(city.Population(), 100))
	return 1.4 * math.Sqrt(hectares/100/math.Pi)
}

// gridZoneAt returns the grid zone covering a point this year, if any.
func (city *City) gridZoneAt(point Point) (GridZone, bool) {
	for _, zone := range city.Scenario.Grid {
		if city.Year < zone.FromYear || len(zone.Area) < 3 {
			continue
		}
		area := make([]Point, 0, len(zone.Area))
		for _, corner := range zone.Area {
			area = append(area, Point{corner[0], corner[1]})
		}
		if polygonGeometry(area).Contains(pointGeometry(point)) {
			return zone, true
		}
	}
	return GridZone{}, false
}

// ---------------------------------------------------------------- parcel index

func (city *City) storeParcel(parcel *Parcel) {
	parcel.Identifier = len(city.Parcels)
	city.Parcels = append(city.Parcels, parcel)
	minimum, maximum := boundsOf(parcel.Polygon)
	for _, cell := range cellsAround(minimum, maximum, 0) {
		if city.parcelCells[cell] == nil {
			city.parcelCells[cell] = map[int]bool{}
		}
		city.parcelCells[cell][parcel.Identifier] = true
	}
}

// parcelsNear returns the polygons of the parcels whose cells touch a box.
func (city *City) parcelsNear(minimum, maximum Point) []*geos.Geom {
	seen := map[int]bool{}
	var polygons []*geos.Geom
	for _, cell := range cellsAround(minimum, maximum, 0) {
		for identifier := range city.parcelCells[cell] {
			if !seen[identifier] {
				seen[identifier] = true
				polygons = append(polygons, city.Parcels[identifier].Polygon)
			}
		}
	}
	return polygons
}

// parcelCountAround counts the parcels in the cells within some cells of a point.
func (city *City) parcelCountAround(point Point, reach int) int {
	centre := cellOf(point)
	count := 0
	for column := -reach; column <= reach; column++ {
		for row := -reach; row <= reach; row++ {
			count += len(city.parcelCells[cellKey{centre.column + column, centre.row + row}])
		}
	}
	return count
}

// removeParcelsIn demolishes the parcels a zone takes, or trims them when most of them is left.
func (city *City) removeParcelsIn(zone *geos.Geom) {
	for _, parcel := range city.Parcels {
		if !parcel.standing() || !parcel.Polygon.Intersects(zone) {
			continue
		}
		rest := parcel.Polygon.Difference(zone)
		if rest.IsEmpty() || rest.Area() < 0.5*parcel.Polygon.Area() {
			parcel.Use, parcel.Coverage, parcel.Floors = UseDemolished, 0, 0
			continue
		}
		if piece := largestPolygon(rest); piece != nil {
			parcel.Polygon = valid(piece)
			parcel.SquareMetres = parcel.Polygon.Area() * 1e6
			if parcel.BuildingDepth > 0 {
				parcel.Coverage = coverageOf(parcel)
			}
		}
	}
}
