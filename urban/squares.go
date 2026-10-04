package urban

import (
	"math"
	"sort"

	"github.com/twpayne/go-geos"
)

// Squares in an organic town are not cut out with a compass: they are widenings where busy streets
// meet, in front of gates and at bridgeheads, irregular and closed (Sitte 1889; Alexander et al.
// 1977, "activity nodes" and "small public squares"). With the scenario's "squares" settings, every
// few years the town opens a square at the busiest crossing that has none nearby, until it has one
// for every so many inhabitants.

// SquareSettings are the scenario's "squares" settings.
type SquareSettings struct {
	PerInhabitants float64    `json:"per_inhabitants"` // one square for every so many people; default 2500
	Spacing        float64    `json:"spacing"`         // km between squares; default 0.2
	Size           [2]float64 `json:"size"`            // reach of a square from its crossing (km); default [0.012, 0.03]
	AtGates        *bool      `json:"at_gates"`        // a square inside every gate; default true
	AtBridges      *bool      `json:"at_bridges"`      // a square at bridgeheads; default true
}

func (settings *SquareSettings) perInhabitants() float64 {
	return positiveOr(settings.PerInhabitants, 2500)
}

func (settings *SquareSettings) spacing() float64 { return positiveOr(settings.Spacing, 0.2) }

func (settings *SquareSettings) size() [2]float64 {
	return pairOr(settings.Size, [2]float64{0.012, 0.03})
}

func yes(value *bool) bool { return value == nil || *value }

// squareCandidate is a place where a square could open, with how much it is wanted.
type squareCandidate struct {
	node   int
	wanted float64
}

// openSquares opens squares where the town wants them most.
func (city *City) openSquares() {
	settings := city.Scenario.Squares
	if settings == nil {
		return
	}
	wanted := int(city.Population() / settings.perInhabitants())
	for _, candidate := range city.squareCandidates() {
		if len(city.Squares) >= wanted+1 {
			return
		}
		if city.nearSquare(city.Network.Nodes[candidate.node], settings.spacing()) {
			continue
		}
		city.openSquareAt(candidate.node, candidate.wanted)
	}
}

// squareCandidates lists the crossings inside the town, the most wanted first: busy streets (local
// integration), gates and bridgeheads.
func (city *City) squareCandidates() []squareCandidate {
	settings := city.Scenario.Squares
	special := city.gateAndBridgeNodes(settings)
	var candidates []squareCandidate
	for node, point := range city.Network.Nodes {
		if city.Network.Degree(node) < 3 && !special[node] {
			continue
		}
		if city.parcelCountAround(point, 1) < 4 || !city.Window.Contains(pointGeometry(point)) {
			continue // a square belongs where people live
		}
		wanted := city.busyness(node)
		if special[node] {
			wanted += 1
		}
		candidates = append(candidates, squareCandidate{node, wanted})
	}
	sort.SliceStable(candidates, func(first, second int) bool { return candidates[first].wanted > candidates[second].wanted })
	return candidates
}

// busyness is the local integration of the streets meeting at a node (or their number, before space syntax runs).
func (city *City) busyness(node int) float64 {
	total := 0.0
	for _, neighbour := range city.Network.sortedNeighbours(node) { // a fixed order: repeatable sums
		for _, identifier := range city.Network.EdgesNear(city.Network.Nodes[node], city.Network.Nodes[node], 0.001) {
			edge := city.Network.Edges[identifier]
			if (edge.From == node && edge.To == neighbour) || (edge.To == node && edge.From == neighbour) {
				total += math.Max(edge.LocalIntegration, 0.1)
			}
		}
	}
	return total
}

// gateAndBridgeNodes are the nodes just inside the gates and at the ends of bridges.
func (city *City) gateAndBridgeNodes(settings *SquareSettings) map[int]bool {
	nodes := map[int]bool{}
	if wall := city.CurrentWall(); wall != nil && yes(settings.AtGates) {
		for _, gate := range wall.Gates {
			if node := city.Network.NearestNode(gate, 0.06, nil); node >= 0 && wall.Polygon.Contains(pointGeometry(city.Network.Nodes[node])) {
				nodes[node] = true
			}
		}
	}
	if yes(settings.AtBridges) {
		for _, bridge := range city.Bridges {
			for _, end := range []Point{bridge.From, bridge.To} {
				if node := city.Network.NearestNode(end, 0.02, nil); node >= 0 {
					nodes[node] = true
				}
			}
		}
	}
	return nodes
}

func (city *City) nearSquare(point Point, spacing float64) bool {
	geometry := pointGeometry(point)
	for _, square := range city.Squares {
		if square.Distance(geometry) < spacing {
			return true
		}
	}
	return false
}

// openSquareAt opens an irregular square at a crossing: the hull of points a little way along every
// street that meets there, so it widens the streets instead of cutting a circle.
func (city *City) openSquareAt(node int, wanted float64) {
	size := city.Scenario.Squares.size()
	reach := lerp(size[0], size[1], math.Min(1, wanted/3))
	centre := city.Network.Nodes[node]
	points := []*geos.Geom{}
	for _, direction := range city.Network.Directions(node) {
		along := reach * uniform(city.random, 0.8, 1.6)
		points = append(points, pointGeometry(centre.Towards(direction+city.random.NormFloat64()*0.15, along)))
	}
	points = append(points, pointGeometry(centre).Buffer(reach*0.6, 4))
	square := unionOf(points...).ConvexHull()
	if square.Area() < 1e-5 || square.Intersects(city.Rivers) || square.Intersects(city.Obstacles) {
		return
	}
	city.Squares = append(city.Squares, square)
	city.removeParcelsIn(square)
}
