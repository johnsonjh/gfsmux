package gfsmux // import "go.gridfinity.dev/gfsmux"

type ShaperHeap []WriteRequest

func (h ShaperHeap) Len() int            { return len(h) }
func (h ShaperHeap) Less(i, j int) bool  { return h[i].Prio < h[j].Prio }
func (h ShaperHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *ShaperHeap) Push(x interface{}) { *h = append(*h, x.(WriteRequest)) }

func (h *ShaperHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
