package urban

import (
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

func TestSegmentsCrossWhereTheyShould(t *testing.T) {
	across := Segment{Point{0, 0}, Point{2, 2}}
	if point, crosses := across.Crossing(Segment{Point{0, 2}, Point{2, 0}}); !crosses || point.DistanceTo(Point{1, 1}) > 1e-12 {
		t.Fatalf("the diagonals cross at (1, 1): %v %v", point, crosses)
	}
	if _, crosses := across.Crossing(Segment{Point{3, 0}, Point{4, 0}}); crosses {
		t.Fatalf("far segments do not cross")
	}
	if projected := (Segment{Point{0, 0}, Point{4, 0}}).Project(Point{1, 5}); projected != 1 {
		t.Fatalf("projection along the segment: %v", projected)
	}
}

func TestNetworkSplitsAndFindsEdges(t *testing.T) {
	network := NewNetwork()
	network.AddPath([]Point{{0, 0}, {0.2, 0}}, ClassRoad, math.Inf(-1), 0.04)
	if len(network.Edges) != 5 || len(network.Nodes) != 6 {
		t.Fatalf("a 0.2 road cut every 0.04 has 5 pieces: %d edges, %d nodes", len(network.Edges), len(network.Nodes))
	}
	first := network.SortedEdgeIdentifiers()[0]
	middle := network.SplitEdge(first, Point{0.02, 0})
	if network.Degree(middle) != 2 || len(network.Edges) != 6 {
		t.Fatalf("splitting adds a node of degree 2")
	}
	if near := network.EdgesNear(Point{0.1, 0}, Point{0.1, 0}, 0.001); len(near) == 0 {
		t.Fatalf("the index should find edges near the road")
	}
	if network.NearestNode(Point{0.041, 0.001}, magnetRadius, nil) != 1 {
		t.Fatalf("the nearest node to (0.041, 0.001) is node 1")
	}
}

// gridNetwork is a square grid of streets, size × size blocks of side 0.1.
func gridNetwork(size int) *Network {
	network := NewNetwork()
	for row := 0; row <= size; row++ {
		network.AddPath([]Point{{0, float64(row) * 0.1}, {float64(size) * 0.1, float64(row) * 0.1}}, ClassStreet, 0, 0.1)
	}
	for column := 0; column <= size; column++ {
		network.AddPath([]Point{{float64(column) * 0.1, 0}, {float64(column) * 0.1, float64(size) * 0.1}}, ClassStreet, 0, 0.1)
	}
	return network
}

func TestTheMiddleOfAGridIsTheMostIntegrated(t *testing.T) {
	network := gridNetwork(6)
	measures, measured := measureSyntax(network, 1000, 10, rand.New(rand.NewPCG(1, 2)))
	if !measured {
		t.Fatal("a grid is big enough to measure")
	}
	best, bestValue := -1, -1.0
	for identifier, value := range measures.integration {
		if value > bestValue {
			best, bestValue = identifier, value
		}
	}
	// following a straight street costs nothing, so the pieces of both central lines tie
	middle := network.Segment(network.Edges[best]).Middle()
	if math.Abs(middle.X-0.3) > 1e-9 && math.Abs(middle.Z-0.3) > 1e-9 {
		t.Fatalf("the most integrated piece should lie on a central line, it is at %v", middle)
	}
}

func TestQuantileInterpolatesLikeNumpy(t *testing.T) {
	if value := quantile([]float64{1, 2, 3, 4}, 0.9); math.Abs(value-3.7) > 1e-12 {
		t.Fatalf("numpy gives 3.7, got %v", value)
	}
}

func TestReliefGridsAreReadFromNumpyFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relief.npy")
	header := "{'descr': '<f8', 'fortran_order': False, 'shape': (2, 3), }"
	for (10+len(header)+1)%64 != 0 {
		header += " "
	}
	header += "\n"
	contents := append([]byte("\x93NUMPY\x01\x00"), byte(len(header)), 0)
	contents = append(contents, header...)
	for _, value := range []float64{0, 10, 20, 0, 10, 20} {
		bits := math.Float64bits(value)
		for shift := 0; shift < 64; shift += 8 {
			contents = append(contents, byte(bits>>shift))
		}
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	terrain, err := LoadTerrain(TerrainFile{Path: path, CellSize: 0.01})
	if err != nil {
		t.Fatal(err)
	}
	if height := terrain.Height(0.015, 0.005); math.Abs(height-15) > 1e-9 {
		t.Fatalf("halfway between 10 and 20 m: %v", height)
	}
	if slope := terrain.Slope(0.01, 0); math.Abs(slope-1) > 1e-9 {
		t.Fatalf("10 m every 10 m is a slope of 1: %v", slope)
	}
}
