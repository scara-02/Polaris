package quadtree

import (
	"sync"
)

type Point struct {
	Lat  float64
	Lon  float64
	Data string
}

type Bounds struct {
	X, Y, Width, Height float64
}

// --- Thread-Safe Wrapper ---

type SafeQuadTree struct {
	mu   sync.RWMutex
	tree *QuadTree
}

func NewSafeQuadTree(bounds Bounds) *SafeQuadTree {
	return &SafeQuadTree{
		tree: newQuadTree(bounds),
	}
}

func (sqt *SafeQuadTree) Insert(p Point) {
	sqt.mu.Lock()
	defer sqt.mu.Unlock()
	sqt.tree.root.insert(p)
}

func (sqt *SafeQuadTree) Search(rangeBounds Bounds) []Point {
	sqt.mu.RLock()
	defer sqt.mu.RUnlock()
	var found []Point
	sqt.tree.root.query(rangeBounds, &found)
	return found
}

func (sqt *SafeQuadTree) Remove(p Point) bool {
    sqt.mu.Lock()
    defer sqt.mu.Unlock()
    return sqt.tree.remove(p)
}

func (qt *QuadTree) remove(p Point) bool {
    return qt.root.remove(p)
}
// --- Pure Algorithm ---

type QuadTree struct {
	root *Node
}

type Node struct {
	Bounds   Bounds
	Points   []Point
	Children []*Node
	Capacity int
	Divided  bool
}

func newQuadTree(bounds Bounds) *QuadTree {
	return &QuadTree{
		root: &Node{
			Bounds:   bounds,
			Capacity: 4,
			Points:   make([]Point, 0),
		},
	}
}

func (n *Node) insert(p Point) bool {
	if !n.Bounds.Contains(p) {
		return false
	}
	if len(n.Points) < n.Capacity && !n.Divided {
		n.Points = append(n.Points, p)
		return true
	}
	if !n.Divided {
		n.subdivide()
	}
	for _, child := range n.Children {
		if child.insert(p) {
			return true
		}
	}
	return false
}

func (n *Node) remove(p Point) bool {
    // 1. Optimization: If point is not in my box, don't bother looking inside
    // p.Lat >= b.X &&
		// p.Lat <= b.X+b.Width &&
		// p.Lon >= b.Y &&
		// p.Lon <= b.Y+b.Height
		if !n.Bounds.Contains(p) {
        return false
    }

    // 2. Check the points I am holding directly
    for i, point := range n.Points {
        // We match by ID (Data) to be sure it's the right driver
				
        if point.Data == p.Data && point.Lat == p.Lat && point.Lon == p.Lon {
            // Delete Trick: Replace the found item with the last item in the list
            // Then chop off the last item. This is O(1) and faster than shifting arrays.
            n.Points[i] = n.Points[len(n.Points)-1]
            n.Points = n.Points[:len(n.Points)-1]
            return true
        }
    }

    // 3. If not found here, check my children
    if n.Divided {
        for _, child := range n.Children {
            if child.remove(p) {
                return true
            }
        }
    }
    return false
}

func (n *Node) subdivide() {
	x, y := n.Bounds.X, n.Bounds.Y
	w, h := n.Bounds.Width/2, n.Bounds.Height/2
	n.Children = []*Node{
		{Bounds: Bounds{x, y, w, h}, Capacity: n.Capacity},
		{Bounds: Bounds{x + w, y, w, h}, Capacity: n.Capacity},
		{Bounds: Bounds{x, y + h, w, h}, Capacity: n.Capacity},
		{Bounds: Bounds{x + w, y + h, w, h}, Capacity: n.Capacity},
	}
	n.Divided = true
}

func (n *Node) query(rangeBounds Bounds, found *[]Point) {
	if !n.Bounds.Intersects(rangeBounds) {
		return
	}
	for _, p := range n.Points {
		if rangeBounds.Contains(p) {
			*found = append(*found, p)
		}
	}
	if n.Divided {
		for _, child := range n.Children {
			child.query(rangeBounds, found)
		}
	}
}

func (b Bounds) Contains(p Point) bool {
	return p.Lat >= b.X && p.Lat <= b.X+b.Width && p.Lon >= b.Y && p.Lon <= b.Y+b.Height
 }

func (b Bounds) Intersects(other Bounds) bool {
	return !(other.X > b.X+b.Width || other.X+other.Width < b.X || other.Y > b.Y+b.Height || other.Y+other.Height < b.Y)
}

