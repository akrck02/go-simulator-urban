package urban

import (
	"container/heap"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
)

// Space syntax by angular segment analysis (Hillier and Iida 2005): the street pieces are the
// nodes of a graph, joined where they meet, and the cost of going from one to the next is the turn
// between them in units of 90°. Choice (how often a piece lies on the cheapest routes) and
// integration (how close a piece is to all the others) are normalised as in Hillier, Yang and
// Turner (2012): NACH = log(CH+1)/log(TD+3) and NAIN = (NC+2)^1.2/(TD+2).

// angularGraph is the street pieces and the turns between them.
type angularGraph struct {
	pieces    []int           // edge identifiers, index = graph node
	positions map[int]int     // edge identifier → graph node
	links     [][]angularLink // per graph node
	middles   []Point         // middle of each piece
}

type angularLink struct {
	to     int
	weight float64
}

func newAngularGraph(network *Network) *angularGraph {
	graph := &angularGraph{positions: map[int]int{}}
	incident := map[int][]int{}
	for _, identifier := range network.SortedEdgeIdentifiers() {
		edge := network.Edges[identifier]
		graph.positions[identifier] = len(graph.pieces)
		graph.pieces = append(graph.pieces, identifier)
		graph.middles = append(graph.middles, network.Segment(edge).Middle())
		incident[edge.From] = append(incident[edge.From], identifier)
		incident[edge.To] = append(incident[edge.To], identifier)
	}
	graph.links = make([][]angularLink, len(graph.pieces))
	nodes := make([]int, 0, len(incident))
	for node := range incident {
		nodes = append(nodes, node)
	}
	slices.Sort(nodes)
	for _, node := range nodes {
		graph.linkPiecesAt(network, node, incident[node])
	}
	return graph
}

// linkPiecesAt joins every pair of pieces that meet at a node, weighted by the turn between them.
func (graph *angularGraph) linkPiecesAt(network *Network, node int, pieces []int) {
	origin := network.Nodes[node]
	leaving := make([]float64, len(pieces))
	for index, identifier := range pieces {
		edge := network.Edges[identifier]
		other := edge.To
		if other == node {
			other = edge.From
		}
		leaving[index] = origin.AngleTo(network.Nodes[other])
	}
	for first := range pieces {
		for second := first + 1; second < len(pieces); second++ {
			turn := math.Pi - math.Abs(angleDifference(leaving[first], leaving[second]))
			weight := turn/(math.Pi/2) + 1e-6
			a, b := graph.positions[pieces[first]], graph.positions[pieces[second]]
			graph.links[a] = append(graph.links[a], angularLink{to: b, weight: weight})
			graph.links[b] = append(graph.links[b], angularLink{to: a, weight: weight})
		}
	}
}

// shortestPaths runs Dijkstra from a source, only through allowed nodes (nil = all). It returns
// the distances (−1 = unreachable), the order in which nodes were settled, the number of cheapest
// paths to each node and the predecessors on them (for Brandes' betweenness).
func (graph *angularGraph) shortestPaths(source int, allowed []bool) (distances []float64, order []int, paths []float64, predecessors [][]int) {
	count := len(graph.pieces)
	distances, paths, predecessors = make([]float64, count), make([]float64, count), make([][]int, count)
	for index := range distances {
		distances[index] = -1
	}
	distances[source], paths[source] = 0, 1
	queue := &distanceQueue{{node: source}}
	settled := make([]bool, count)
	for queue.Len() > 0 {
		current := heap.Pop(queue).(queued)
		if settled[current.node] {
			continue
		}
		settled[current.node] = true
		order = append(order, current.node)
		for _, link := range graph.links[current.node] {
			if allowed != nil && !allowed[link.to] {
				continue
			}
			graph.relax(current, link, distances, paths, predecessors, queue)
		}
	}
	return distances, order, paths, predecessors
}

const sameDistance = 1e-12

func (graph *angularGraph) relax(current queued, link angularLink, distances, paths []float64, predecessors [][]int, queue *distanceQueue) {
	candidate := current.distance + link.weight
	known := distances[link.to]
	switch {
	case known < 0 || candidate < known-sameDistance:
		distances[link.to], paths[link.to], predecessors[link.to] = candidate, paths[current.node], []int{current.node}
		heap.Push(queue, queued{node: link.to, distance: candidate})
	case math.Abs(candidate-known) <= sameDistance:
		paths[link.to] += paths[current.node]
		predecessors[link.to] = append(predecessors[link.to], current.node)
	}
}

// syntaxMeasures are the results for every street piece.
type syntaxMeasures struct {
	choice, integration, localIntegration map[int]float64
}

// measureSyntax samples sources (like networkx with k) to estimate total depth and choice, and
// computes local integration within a radius around each piece.
func measureSyntax(network *Network, samples int, localRadius float64, random *rand.Rand) (syntaxMeasures, bool) {
	graph := newAngularGraph(network)
	count := len(graph.pieces)
	if count < 10 {
		return syntaxMeasures{}, false
	}
	sources := random.Perm(count)[:min(samples, count)]
	totalDepth, betweenness := make([]float64, count), make([]float64, count)
	for _, source := range sources {
		distances, order, paths, predecessors := graph.shortestPaths(source, nil)
		for node, distance := range distances {
			if distance > 0 {
				totalDepth[node] += distance
			}
		}
		accumulateBetweenness(betweenness, source, order, paths, predecessors)
	}
	return graph.normalise(network, totalDepth, betweenness, len(sources), localRadius), true
}

// accumulateBetweenness adds one source's dependencies (Brandes 2001).
func accumulateBetweenness(betweenness []float64, source int, order []int, paths []float64, predecessors [][]int) {
	dependency := make([]float64, len(betweenness))
	for index := len(order) - 1; index >= 0; index-- {
		node := order[index]
		for _, previous := range predecessors[node] {
			dependency[previous] += paths[previous] / paths[node] * (1 + dependency[node])
		}
		if node != source {
			betweenness[node] += dependency[node]
		}
	}
}

func (graph *angularGraph) normalise(network *Network, totalDepth, betweenness []float64, sourceCount int, localRadius float64) syntaxMeasures {
	count := len(graph.pieces)
	factor := float64(count) / float64(sourceCount)
	measures := syntaxMeasures{choice: map[int]float64{}, integration: map[int]float64{}, localIntegration: map[int]float64{}}
	for node, identifier := range graph.pieces {
		depth := totalDepth[node] * factor
		// networkx rescales sampled betweenness by n/k and halves it for undirected graphs; the
		// Python simulator then multiplied by n/k again. Kept as it was, so results compare.
		choice := betweenness[node] * 0.5 * factor * factor
		measures.choice[identifier] = math.Log(choice+1) / math.Log(depth+3)
		measures.integration[identifier] = math.Pow(float64(count+2), 1.2) / (depth + 2)
	}
	graph.measureLocalIntegration(network, localRadius, measures.localIntegration)
	return measures
}

// measureLocalIntegration: integration within a straight-line radius of each piece's middle.
func (graph *angularGraph) measureLocalIntegration(network *Network, radius float64, results map[int]float64) {
	for node, identifier := range graph.pieces {
		middle := graph.middles[node]
		allowed := make([]bool, len(graph.pieces))
		for _, nearby := range network.EdgesNear(middle, middle, radius) {
			if position, known := graph.positions[nearby]; known && graph.middles[position].DistanceTo(middle) <= radius {
				allowed[position] = true
			}
		}
		allowed[node] = true
		distances, _, _, _ := graph.shortestPaths(node, allowed)
		reached, total := 0, 0.0
		for _, distance := range distances {
			if distance >= 0 {
				reached++
				total += distance
			}
		}
		results[identifier] = math.Pow(float64(reached+2), 1.2) / (total + 2)
	}
}

// quantile returns the value under which a share of the values fall (linear, like numpy).
func quantile(values []float64, share float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	sort.Float64s(sorted)
	position := share * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := min(lower+1, len(sorted)-1)
	return sorted[lower] + (sorted[upper]-sorted[lower])*(position-float64(lower))
}

// ---------------------------------------------------------------- priority queue

type queued struct {
	node     int
	distance float64
}

type distanceQueue []queued

func (queue distanceQueue) Len() int { return len(queue) }
func (queue distanceQueue) Less(first, second int) bool {
	return queue[first].distance < queue[second].distance
}
func (queue distanceQueue) Swap(first, second int) {
	queue[first], queue[second] = queue[second], queue[first]
}
func (queue *distanceQueue) Push(value any) { *queue = append(*queue, value.(queued)) }
func (queue *distanceQueue) Pop() any {
	old := *queue
	last := old[len(old)-1]
	*queue = old[:len(old)-1]
	return last
}
