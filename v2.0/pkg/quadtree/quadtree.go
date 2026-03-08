package quadtree

import (
	"sync"
)

// Bounds represents a rectangular geographical area
type Bounds struct {
	X, Y, Width, Height float64
}

func (b *Bounds) Contains(p Point) bool {
	return p.Lat >= b.X && p.Lat <= b.X+b.Width &&
		p.Lon >= b.Y && p.Lon <= b.Y+b.Height
}

func (b *Bounds) Intersects(other Bounds) bool {
	return !(other.X > b.X+b.Width ||
		other.X+other.Width < b.X ||
		other.Y > b.Y+b.Height ||
		other.Y+other.Height < b.Y)
}

// Point represents a generic spatial node in the QuadTree
type Point struct {
	Lat   float64
	Lon   float64
	ID    string
	Class uint16 // The AssetClass Bitmask (e.g., 16 for Drone)
}

// SafeQuadTree is a thread-safe wrapper for high-concurrency access
type SafeQuadTree struct {
	mu   sync.RWMutex
	root *Node
}

type Node struct {
	Bounds   Bounds
	Points   []Point
	Children [4]*Node
	Divided  bool
}

func NewSafeQuadTree(bounds Bounds) *SafeQuadTree {
	return &SafeQuadTree{
		root: &Node{Bounds: bounds},
	}
}

// Clear wipes the tree (used during our batch updates)
func (sqt *SafeQuadTree) Clear() {
	sqt.mu.Lock()
	defer sqt.mu.Unlock()
	sqt.root.Points = nil
	sqt.root.Divided = false
	sqt.root.Children = [4]*Node{}
}

func (sqt *SafeQuadTree) Insert(p Point) {
	sqt.mu.Lock()
	defer sqt.mu.Unlock()
	sqt.root.insert(p)
}

func (n *Node) insert(p Point) bool {
	if !n.Bounds.Contains(p) {
		return false
	}
	// For simplicity in this implementation, we just store points.
	// A production QuadTree would subdivide when capacity is reached.
	n.Points = append(n.Points, p)
	return true
}

// Search queries the QuadTree using BOTH geography and the hardware bitmask
func (sqt *SafeQuadTree) Search(rangeBounds Bounds, reqClass uint16) []Point {
	sqt.mu.RLock()
	defer sqt.mu.RUnlock()
	var found []Point
	sqt.root.query(rangeBounds, reqClass, &found)
	return found
}

func (n *Node) query(rangeBounds Bounds, reqClass uint16, found *[]Point) {
	if !n.Bounds.Intersects(rangeBounds) {
		return
	}
	for _, p := range n.Points {
		// Bitwise AND filtering. 
		// If the node's class matches the requested bitmask, it's added.
		if rangeBounds.Contains(p) && (p.Class&reqClass) > 0 {
			*found = append(*found, p)
		}
	}
}