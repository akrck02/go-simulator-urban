package urban

import (
	"math"

	"github.com/twpayne/go-geos"
)

const (
	mainStreetShare     = 0.9 // the 10 % of pieces with most choice become main streets
	shopShare           = 0.8 // shops go to the 20 % of pieces with most local integration
	marketRadius        = 0.035
	marketSpacing       = 0.15
	populationForMarket = 5000
)

// applySyntax measures the street network and lets the result shape the city: main streets,
// the market square on the most integrated piece and shops on the locally integrated ones
// (natural movement, Hillier et al. 1993).
func (city *City) applySyntax(samples int, localRadius float64) {
	measures, measured := measureSyntax(city.Network, samples, localRadius, city.random)
	if !measured {
		return
	}
	choices, locals := make([]float64, 0, len(measures.choice)), make([]float64, 0, len(measures.localIntegration))
	for _, identifier := range city.Network.SortedEdgeIdentifiers() {
		edge := city.Network.Edges[identifier]
		edge.Choice, edge.Integration = measures.choice[identifier], measures.integration[identifier]
		edge.LocalIntegration, edge.hasLocalIntegration = measures.localIntegration[identifier], true
		choices, locals = append(choices, edge.Choice), append(locals, edge.LocalIntegration)
	}
	city.promoteMainStreets(quantile(choices, mainStreetShare))
	city.openMarketSquare()
	city.openShops(quantile(locals, shopShare))
}

func (city *City) promoteMainStreets(threshold float64) {
	for _, edge := range city.Network.Edges {
		if edge.Choice >= threshold && (edge.Class == ClassStreet || edge.Class == ClassAlley) {
			edge.Class = ClassMain
		}
	}
}

// openMarketSquare opens a square in the middle of the most integrated street piece, once the city
// is big enough and if there is no square near it yet (two squares at most).
func (city *City) openMarketSquare() {
	var best *Edge
	for _, identifier := range city.Network.SortedEdgeIdentifiers() {
		if edge := city.Network.Edges[identifier]; best == nil || edge.Integration > best.Integration {
			best = edge
		}
	}
	if best == nil || len(city.Squares) >= 2 || city.Population() <= populationForMarket {
		return
	}
	middle := pointGeometry(city.Network.Segment(best).Middle())
	for _, square := range city.Squares {
		if square.Distance(middle) <= marketSpacing {
			return
		}
	}
	square := middle.Buffer(marketRadius, 8)
	city.Squares = append(city.Squares, square)
	city.removeParcelsIn(square)
}

func (city *City) openShops(threshold float64) {
	for _, parcel := range city.Parcels {
		edge, exists := city.Network.Edges[parcel.Edge]
		if exists && parcel.Use == UseHousing && edge.hasLocalIntegration && edge.LocalIntegration >= threshold {
			parcel.Use = UseShop
		}
	}
}

// ---------------------------------------------------------------- buildings

// Footprint is the building of a plot: it takes the front and grows towards the back with the
// coverage (burgage cycle); extensive plots have a round building in the middle.
func (parcel *Parcel) Footprint() *geos.Geom {
	if parcel.Use == UseExtensive {
		radius := math.Sqrt(parcel.Polygon.Area() * 0.2 / math.Pi)
		return parcel.Polygon.Centroid().Buffer(radius, 8)
	}
	minimum, maximum := boundsOf(parcel.Polygon)
	far := 2 * maximum.DistanceTo(minimum)
	base := math.Inf(1)
	for _, corner := range coordinatesOf(parcel.Polygon.ExteriorRing()) {
		base = math.Min(base, corner.Dot(parcel.Normal))
	}
	depth := parcel.Depth * math.Min(1, parcel.Coverage/fullCoverage*0.85+0.15)
	if parcel.BuildingDepth > 0 {
		depth = parcel.BuildingDepth
	}
	centre := centroidOf(parcel.Polygon)
	start := base - centre.Dot(parcel.Normal)
	strip := polygonGeometry([]Point{
		centre.Subtract(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(start)),
		centre.Add(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(start)),
		centre.Add(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(start + depth)),
		centre.Subtract(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(start + depth)),
	})
	built := parcel.Polygon.Intersection(strip)
	if parcel.BackDepth > 0 {
		back := start + parcel.Depth - parcel.BackDepth
		built = built.Union(parcel.Polygon.Intersection(polygonGeometry([]Point{
			centre.Subtract(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(back)),
			centre.Add(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(back)),
			centre.Add(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(start + parcel.Depth)),
			centre.Subtract(parcel.Direction.Scale(far)).Add(parcel.Normal.Scale(start + parcel.Depth)),
		})))
	}
	// a thin inset keeps neighbouring houses apart without a gap: they share walls in a row
	return built.BufferWithStyle(-0.0001, 8, geos.BufCapStyleFlat, geos.BufJoinStyleMitre, 5)
}
