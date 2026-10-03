package urban

import (
	"math"

	"github.com/twpayne/go-geos"
)

// Indicators are measures of the city's form to compare with historic town centres (see the
// published references): settled area against Cesaretti et al. (2016),
// street network against Crucitti, Latora and Porta (2006) and Barthelemy and Flammini (2008),
// blocks against Siksna (1997), plots against Conzen (1960).
type Indicators struct {
	Year               float64 `json:"year"`
	Inhabitants        float64 `json:"inhabitants"`
	SettledHectares    float64 `json:"settled_hectares"`
	InhabitantsPerHa   float64 `json:"inhabitants_per_hectare"`
	ExpectedHectares   float64 `json:"expected_hectares_cesaretti"`
	CrossingsPerHa     float64 `json:"crossings_per_hectare"`
	MeanStreetMetres   float64 `json:"mean_street_piece_metres"`
	EdgesPerNode       float64 `json:"edges_per_node"`
	ShareOfDegreeThree float64 `json:"share_of_degree_three"`
	ShareOfDeadEnds    float64 `json:"share_of_dead_ends"`
	MeanBlockMetres    float64 `json:"mean_block_metres"`
	MeanFrontMetres    float64 `json:"mean_front_metres"`
	MeanDepthMetres    float64 `json:"mean_depth_metres"`
	Coverage           float64 `json:"coverage_gsi"`
	FloorAreaRatio     float64 `json:"floor_area_ratio_far"`
	Parcels            int     `json:"parcels"`
	StreetPieces       int     `json:"street_pieces"`
	Walls              int     `json:"walls"`
}

// Measure computes the indicators over the settled zone (plots and squares, grown by 15 m).
func (city *City) Measure() Indicators {
	var standing []*Parcel
	pieces := append([]*geos.Geom(nil), city.Squares...)
	for _, parcel := range city.Parcels {
		if parcel.standing() {
			standing = append(standing, parcel)
			pieces = append(pieces, parcel.Polygon)
		}
	}
	zone := unionOf(pieces...).Buffer(0.015, 8)
	hectares := zone.Area() * 100
	measured := Indicators{Year: city.Year, Inhabitants: math.Round(city.Population()), SettledHectares: round(hectares, 1),
		InhabitantsPerHa: round(city.Population()/math.Max(hectares, 1e-6), 1), ExpectedHectares: round(expectedHectares(city.Population()), 1),
		Parcels: len(standing), StreetPieces: len(city.Network.Edges), Walls: len(city.Walls)}
	city.measureNetwork(zone, hectares, &measured)
	city.measureBlocks(zone, &measured)
	measureParcels(standing, zone, &measured)
	return measured
}

// measureNetwork: crossings and street pieces inside the zone; nodes of degree 2 are not crossings.
func (city *City) measureNetwork(zone *geos.Geom, hectares float64, measured *Indicators) {
	streetCount, passingNodes, totalLength := 0, 0, 0.0
	for _, edge := range city.Network.Edges {
		segment := city.Network.Segment(edge)
		if zone.Intersects(segmentGeometry(segment)) {
			streetCount++
			totalLength += segment.Length()
		}
	}
	var degrees []int
	for node, point := range city.Network.Nodes {
		degree := city.Network.Degree(node)
		if degree == 0 || !zone.Contains(pointGeometry(point)) {
			continue
		}
		if degree == 2 {
			passingNodes++
			continue
		}
		degrees = append(degrees, degree)
	}
	crossings, degreeThree, deadEnds := 0, 0, 0
	for _, degree := range degrees {
		crossings += boolToInt(degree >= 3)
		degreeThree += boolToInt(degree == 3)
		deadEnds += boolToInt(degree == 1)
	}
	nodeCount := math.Max(float64(len(degrees)), 1)
	measured.CrossingsPerHa = round(float64(crossings)/math.Max(hectares, 1e-6), 2)
	if streetCount > 0 {
		measured.MeanStreetMetres = round(totalLength/math.Max(float64(streetCount-passingNodes), 1)*1000, 1)
	}
	measured.EdgesPerNode = round(float64(streetCount-passingNodes)/nodeCount, 2)
	measured.ShareOfDegreeThree = round(float64(degreeThree)/nodeCount, 2)
	measured.ShareOfDeadEnds = round(float64(deadEnds)/nodeCount, 2)
}

func (city *City) measureBlocks(zone *geos.Geom, measured *Indicators) {
	total, count := 0.0, 0
	for _, block := range city.Blocks() {
		if zone.Contains(block.Centroid()) {
			total += math.Sqrt(block.Area())
			count++
		}
	}
	if count > 0 {
		measured.MeanBlockMetres = round(total/float64(count)*1000, 1)
	}
}

func measureParcels(standing []*Parcel, zone *geos.Geom, measured *Indicators) {
	builtGround, floorSpace, front, depth, homes := 0.0, 0.0, 0.0, 0.0, 0
	for _, parcel := range standing {
		if footprint := parcel.CachedFootprint(); !footprint.IsEmpty() {
			builtGround += footprint.Area() * 1e6
		}
		floorSpace += parcel.BuiltSquareMetres()
		if parcel.Use == UseHousing || parcel.Use == UseShop || parcel.Use == UseWorkshop {
			front += parcel.Front
			depth += parcel.Depth
			homes++
		}
	}
	zoneSquareMetres := math.Max(zone.Area()*1e6, 1)
	measured.Coverage = round(builtGround/zoneSquareMetres, 2)
	measured.FloorAreaRatio = round(floorSpace/zoneSquareMetres, 2)
	if homes > 0 {
		measured.MeanFrontMetres = round(front/float64(homes)*1000, 1)
		measured.MeanDepthMetres = round(depth/float64(homes)*1000, 1)
	}
}

func round(value float64, decimals int) float64 {
	scale := math.Pow(10, float64(decimals))
	return math.Round(value*scale) / scale
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
