package urban

import (
	"math"
	"slices"
)

// Street classes, from the widest to the narrowest.
const (
	ClassRoad   = "road" // roads that reach the city from outside
	ClassMain   = "main" // the streets most people go through (space syntax)
	ClassRing   = "ring" // ring roads where old walls stood
	ClassStreet = "street"
	ClassAlley  = "alley"
)

// streetWidths are the widths of each class (world units, km).
var streetWidths = map[string]float64{ClassRoad: 0.008, ClassMain: 0.009, ClassRing: 0.010, ClassStreet: 0.006, ClassAlley: 0.0035}

func widthOf(class string) float64 {
	if width, known := streetWidths[class]; known {
		return width
	}
	return 0.006
}

const (
	indexCell    = 0.05  // cell of the spatial index
	magnetRadius = 0.025 // a new street snaps to a node this close (Weber et al. 2009)
)

// Edge is a straight piece of street between two nodes.
type Edge struct {
	Identifier          int
	From, To            int
	Class               string
	Year                float64 // when it was laid out (−Inf for the roads that were already there)
	Choice              float64 // normalised choice (NACH)
	Integration         float64 // normalised integration (NAIN), global
	LocalIntegration    float64 // normalised integration within the local radius
	hasLocalIntegration bool
}

// cellKey identifies a cell of the spatial index.
type cellKey struct{ column, row int }

// Network is a planar graph of straight street pieces with a spatial index.
type Network struct {
	Nodes      []Point
	neighbours []map[int]bool
	failures   []int // failed attempts to open a street from each node
	Edges      map[int]*Edge
	nextEdge   int
	edgeCells  map[cellKey]map[int]bool
	nodeCells  map[cellKey][]int
}

// NewNetwork returns an empty network.
func NewNetwork() *Network {
	return &Network{Edges: map[int]*Edge{}, edgeCells: map[cellKey]map[int]bool{}, nodeCells: map[cellKey][]int{}}
}

func cellOf(point Point) cellKey {
	return cellKey{int(math.Floor(point.X / indexCell)), int(math.Floor(point.Z / indexCell))}
}

// cellsAround lists the cells a box (grown by a margin) touches.
func cellsAround(minimum, maximum Point, margin float64) []cellKey {
	low, high := cellOf(Point{minimum.X - margin, minimum.Z - margin}), cellOf(Point{maximum.X + margin, maximum.Z + margin})
	var cells []cellKey
	for column := low.column; column <= high.column; column++ {
		for row := low.row; row <= high.row; row++ {
			cells = append(cells, cellKey{column, row})
		}
	}
	return cells
}

func boxOf(segment Segment) (Point, Point) {
	return Point{math.Min(segment.From.X, segment.To.X), math.Min(segment.From.Z, segment.To.Z)},
		Point{math.Max(segment.From.X, segment.To.X), math.Max(segment.From.Z, segment.To.Z)}
}

// AddNode adds a node and returns its index.
func (network *Network) AddNode(point Point) int {
	index := len(network.Nodes)
	network.Nodes = append(network.Nodes, point)
	network.neighbours = append(network.neighbours, map[int]bool{})
	network.failures = append(network.failures, 0)
	cell := cellOf(point)
	network.nodeCells[cell] = append(network.nodeCells[cell], index)
	return index
}

// NearestNode returns the closest node within a radius, skipping some, or −1.
func (network *Network) NearestNode(point Point, radius float64, skip map[int]bool) int {
	best, bestDistance := -1, radius
	for _, cell := range cellsAround(point, point, radius) {
		for _, index := range network.nodeCells[cell] {
			if skip[index] {
				continue
			}
			if distance := point.DistanceTo(network.Nodes[index]); distance < bestDistance {
				best, bestDistance = index, distance
			}
		}
	}
	return best
}

// Degree returns how many streets meet at a node.
func (network *Network) Degree(node int) int { return len(network.neighbours[node]) }

// Segment returns the geometry of an edge.
func (network *Network) Segment(edge *Edge) Segment {
	return Segment{network.Nodes[edge.From], network.Nodes[edge.To]}
}

// AddEdge joins two nodes; it returns nil when they are the same or already joined.
func (network *Network) AddEdge(from, to int, class string, year float64) *Edge {
	if from == to || network.neighbours[from][to] {
		return nil
	}
	edge := &Edge{Identifier: network.nextEdge, From: from, To: to, Class: class, Year: year}
	network.nextEdge++
	network.Edges[edge.Identifier] = edge
	network.neighbours[from][to], network.neighbours[to][from] = true, true
	minimum, maximum := boxOf(network.Segment(edge))
	for _, cell := range cellsAround(minimum, maximum, 0) {
		if network.edgeCells[cell] == nil {
			network.edgeCells[cell] = map[int]bool{}
		}
		network.edgeCells[cell][edge.Identifier] = true
	}
	return edge
}

// RemoveEdge takes an edge out of the network.
func (network *Network) RemoveEdge(identifier int) *Edge {
	edge := network.Edges[identifier]
	delete(network.Edges, identifier)
	delete(network.neighbours[edge.From], edge.To)
	delete(network.neighbours[edge.To], edge.From)
	minimum, maximum := boxOf(network.Segment(edge))
	for _, cell := range cellsAround(minimum, maximum, 0) {
		delete(network.edgeCells[cell], identifier)
	}
	return edge
}

// SplitEdge cuts an edge at a point and returns the new node.
func (network *Network) SplitEdge(identifier int, point Point) int {
	edge := network.RemoveEdge(identifier)
	middle := network.AddNode(point)
	network.AddEdge(edge.From, middle, edge.Class, edge.Year)
	network.AddEdge(middle, edge.To, edge.Class, edge.Year)
	return middle
}

// EdgesNear returns the edges whose cells touch a box grown by a margin, sorted by identifier.
func (network *Network) EdgesNear(minimum, maximum Point, margin float64) []int {
	seen := map[int]bool{}
	for _, cell := range cellsAround(minimum, maximum, margin) {
		for identifier := range network.edgeCells[cell] {
			if network.Edges[identifier] != nil {
				seen[identifier] = true
			}
		}
	}
	identifiers := make([]int, 0, len(seen))
	for identifier := range seen {
		identifiers = append(identifiers, identifier)
	}
	slices.Sort(identifiers)
	return identifiers
}

// EdgesNearSegment returns the edges near a segment.
func (network *Network) EdgesNearSegment(segment Segment, margin float64) []int {
	minimum, maximum := boxOf(segment)
	return network.EdgesNear(minimum, maximum, margin)
}

// Directions returns the directions of the streets leaving a node.
func (network *Network) Directions(node int) []float64 {
	origin := network.Nodes[node]
	directions := make([]float64, 0, len(network.neighbours[node]))
	for _, neighbour := range network.sortedNeighbours(node) {
		directions = append(directions, origin.AngleTo(network.Nodes[neighbour]))
	}
	return directions
}

func (network *Network) sortedNeighbours(node int) []int {
	neighbours := make([]int, 0, len(network.neighbours[node]))
	for neighbour := range network.neighbours[node] {
		neighbours = append(neighbours, neighbour)
	}
	slices.Sort(neighbours)
	return neighbours
}

// AddPath inserts a polyline as chained pieces, cut every step (the roads that already exist).
func (network *Network) AddPath(points []Point, class string, year, step float64) {
	points = densify(points, step)
	previous := network.NearestNode(points[0], magnetRadius, nil)
	if previous < 0 {
		previous = network.AddNode(points[0])
	}
	for _, point := range points[1:] {
		current := network.NearestNode(point, 0.004, nil)
		if current < 0 {
			current = network.AddNode(point)
		}
		network.AddEdge(previous, current, class, year)
		previous = current
	}
}

// densify adds points so no piece of a polyline is longer than step (like shapely's segmentize).
func densify(points []Point, step float64) []Point {
	result := []Point{points[0]}
	for index := 1; index < len(points); index++ {
		segment := Segment{points[index-1], points[index]}
		pieces := max(1, int(math.Ceil(segment.Length()/step)))
		for piece := 1; piece <= pieces; piece++ {
			result = append(result, segment.At(float64(piece)/float64(pieces)))
		}
	}
	return result
}

// SortedEdgeIdentifiers returns every edge identifier in increasing order, for a stable iteration.
func (network *Network) SortedEdgeIdentifiers() []int {
	identifiers := make([]int, 0, len(network.Edges))
	for identifier := range network.Edges {
		identifiers = append(identifiers, identifier)
	}
	slices.Sort(identifiers)
	return identifiers
}
