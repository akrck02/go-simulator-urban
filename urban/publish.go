package urban

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/akrck02/go-simulator/plugin"
	"github.com/akrck02/go-simulator/world"
	"github.com/twpayne/go-geos"
)

// The city is shown as entities with ground shapes, one batch per kind so thousands of houses
// stay cheap to draw. Heights are in world units (km), exaggerated by the scenario's height_scale.

const (
	metresPerFloor = 3.2
	roofMetres     = 2
	wallHeight     = 11 // metres
	wallWidth      = 0.018
	towerHeight    = 18 // metres
	towerSide      = 0.016
	oldWallWidth   = 0.006
	bridgeWidth    = 0.007
	streetLift     = 0.0006
	squareLift     = 0.0007
)

// colours of the map sheets (urbanismo.py), as linear RGBA.
var colours = map[string][4]float32{
	"garden": linear("#c3c29a"), UseExtensive: linear("#9aa37a"), UseHousing: linear("#b86f52"), UseShop: linear("#c98f6e"),
	UseWorkshop: linear("#6b5a4c"), "street": linear("#e9e0c8"), "wall": linear("#fbf8f0"), "square": linear("#efe7d1"),
	"old_wall": linear("#d9cfb5"), "bridge": linear("#efe7d2"),
}

// linear turns an sRGB hex colour into linear RGBA, the colour space glTF scenes use.
func linear(hex string) [4]float32 {
	var colour [4]float32
	for channel := range 3 {
		value, _ := strconv.ParseUint(hex[1+2*channel:3+2*channel], 16, 8)
		srgb := float64(value) / 255
		if srgb <= 0.04045 {
			colour[channel] = float32(srgb / 12.92)
		} else {
			colour[channel] = float32(math.Pow((srgb+0.055)/1.055, 2.4))
		}
	}
	colour[3] = 1
	return colour
}

// publish sends the changes between what was last sent and the city as it is now.
func (city *City) publish(context *plugin.Context) {
	wanted := city.entities()
	identifiers := make([]string, 0, len(wanted))
	for identifier := range wanted {
		identifiers = append(identifiers, identifier)
	}
	slices.Sort(identifiers)
	var newPlots [][][2]float32
	for _, identifier := range identifiers {
		entity := wanted[identifier]
		if city.publishEntity(context, entity) && entity.Kind == "parcel" {
			newPlots = append(newPlots, entity.Shape.Outline)
		}
	}
	if len(newPlots) > 0 && len(city.Scenario.TreeNodes) > 0 {
		context.ClearSceneArea(&world.Area{Outlines: newPlots, Nodes: city.Scenario.TreeNodes})
	}
	gone := make([]string, 0)
	for identifier := range city.published {
		if _, still := wanted[identifier]; !still {
			gone = append(gone, identifier)
		}
	}
	slices.Sort(gone)
	for _, identifier := range gone {
		context.Remove(world.Identifier(identifier))
		delete(city.published, identifier)
	}
}

// publishEntity sends an entity that is new or changed; it tells whether the entity is new.
func (city *City) publishEntity(context *plugin.Context, entity *world.Entity) bool {
	identifier := string(entity.Identifier)
	signature := signatureOf(entity)
	previous, sent := city.published[identifier]
	switch {
	case !sent:
		context.Add(entity)
	case previous != signature:
		context.Modify(entity)
	default:
		return false
	}
	city.published[identifier] = signature
	return !sent
}

// clearOldScene hides what the drawn scene had under the "clear" features: the old town and
// its trees give way to the simulated city.
func (city *City) clearOldScene(context *plugin.Context) {
	nodes := append(append([]string(nil), city.Scenario.ClearNodes...), city.Scenario.TreeNodes...)
	if city.ClearArea.IsEmpty() || len(nodes) == 0 {
		return
	}
	area := &world.Area{Nodes: nodes}
	for _, polygon := range polygonsOf(city.ClearArea) {
		area.Outlines = append(area.Outlines, outlineOf(polygon))
		for hole := range polygon.NumInteriorRings() {
			area.Keep = append(area.Keep, ringOf(polygon.InteriorRing(hole)))
		}
	}
	context.ClearSceneArea(area)
}

// signatureOf sums up what is shown of an entity, to send only what changed (maps print sorted).
func signatureOf(entity *world.Entity) string {
	shape := entity.Shape
	return fmt.Sprintf("%s|%v|%v|%g|%g|%v", entity.Kind, shape.Outline, shape.Line, shape.Width, shape.Height, entity.Properties)
}

// entities describes the whole city as entities, by identifier.
func (city *City) entities() map[string]*world.Entity {
	entities := map[string]*world.Entity{}
	add := func(entity *world.Entity) { entities[string(entity.Identifier)] = entity }
	for _, identifier := range city.Network.SortedEdgeIdentifiers() {
		add(city.streetEntity(city.Network.Edges[identifier]))
	}
	for _, parcel := range city.Parcels {
		if parcel.standing() {
			add(city.parcelEntity(parcel))
			if building := city.buildingEntity(parcel); building != nil {
				add(building)
			}
		}
	}
	city.addWallEntities(add)
	for index, square := range city.Squares {
		add(groundEntity(fmt.Sprintf("%s-square-%d", city.Name, index), "square", square, 0, "squares", colours["square"], squareLift, nil))
	}
	for index, bridge := range city.Bridges {
		add(lineEntity(fmt.Sprintf("%s-bridge-%d", city.Name, index), "bridge", []Point{bridge.From, bridge.To}, bridgeWidth, 0, "bridges", colours["bridge"], nil))
	}
	return entities
}

func (city *City) streetEntity(edge *Edge) *world.Entity {
	segment := city.Network.Segment(edge)
	properties := map[string]any{"class": edge.Class, "from_node": edge.From, "to_node": edge.To}
	if !math.IsInf(edge.Year, -1) {
		properties["year"] = edge.Year
	}
	return lineEntity(fmt.Sprintf("%s-street-%d", city.Name, edge.Identifier), "street", []Point{segment.From, segment.To},
		widthOf(edge.Class), 0, "streets", colours["street"], properties)
}

func (city *City) parcelEntity(parcel *Parcel) *world.Entity {
	colour := colours["garden"]
	if parcel.Use == UseExtensive {
		colour = colours[UseExtensive]
	}
	properties := map[string]any{"use": parcel.Use, "coverage": parcel.Coverage, "floors": parcel.Floors, "year": parcel.Year,
		"front": parcel.Front, "depth": parcel.Depth, "street": parcel.Edge, "square_metres": parcel.SquareMetres}
	return groundEntity(fmt.Sprintf("%s-parcel-%d", city.Name, parcel.Identifier), "parcel", parcel.Polygon, 0, "parcels", colour, 0, properties)
}

func (city *City) buildingEntity(parcel *Parcel) *world.Entity {
	footprint := largestPolygon(parcel.CachedFootprint())
	if footprint == nil || footprint.Area() == 0 {
		return nil
	}
	height := city.metres(metresPerFloor*float64(parcel.Floors) + roofMetres)
	colour, known := colours[parcel.Use]
	if !known {
		colour = colours[UseHousing]
	}
	return groundEntity(fmt.Sprintf("%s-building-%d", city.Name, parcel.Identifier), "building", footprint, height, "buildings", colour, 0,
		map[string]any{"use": parcel.Use, "floors": parcel.Floors, "parcel": parcel.Identifier})
}

func (city *City) addWallEntities(add func(*world.Entity)) {
	for index, wall := range city.Walls {
		current := index == len(city.Walls)-1
		for part, line := range linesOf(wall.Line) {
			identifier := fmt.Sprintf("%s-wall-%d-%d", city.Name, index, part)
			properties := map[string]any{"year": wall.Year, "current": current}
			if current {
				add(lineEntity(identifier, "wall", coordinatesOf(line), wallWidth, city.metres(wallHeight), "walls", colours["wall"], properties))
			} else {
				add(lineEntity(identifier, "old_wall", coordinatesOf(line), oldWallWidth, 0, "old_walls", colours["old_wall"], properties))
			}
		}
	}
}

// metres turns a height in metres into world units, exaggerated like the rest of the scene.
func (city *City) metres(value float64) float32 {
	return float32(value / 1000 * city.Scenario.HeightScale)
}

func groundEntity(identifier, kind string, polygon *geos.Geom, height float32, batch string, colour [4]float32, lift float32, properties map[string]any) *world.Entity {
	shape := &world.Shape{Outline: outlineOf(polygon), Height: height, Batch: batch, Color: &colour, Lift: lift}
	return &world.Entity{Identifier: world.Identifier(identifier), Kind: kind, Shape: shape, Properties: properties}
}

func lineEntity(identifier, kind string, points []Point, width float64, height float32, batch string, colour [4]float32, properties map[string]any) *world.Entity {
	line := make([][2]float32, 0, len(points))
	for _, point := range points {
		line = append(line, [2]float32{float32(point.X), float32(point.Z)})
	}
	shape := &world.Shape{Line: line, Width: float32(width), Height: height, Batch: batch, Color: &colour, Lift: streetLift}
	return &world.Entity{Identifier: world.Identifier(identifier), Kind: kind, Shape: shape, Properties: properties}
}
