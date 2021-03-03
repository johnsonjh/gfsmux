package gfsmux

// ShaperHeap ...
type ShaperHeap []WriteRequest

// Len ...
func (
	h ShaperHeap,
) Len() int {
	return len(
		h,
	)
}

// Less ...
func (
	h ShaperHeap,
) Less(
	i,
	j int,
) bool {
	return h[i].Prio < h[j].Prio
}

// Swap ...
func (
	h ShaperHeap,
) Swap(
	i,
	j int,
) {
	h[i], h[j] = h[j], h[i]
}

// Push ...
func (
	h *ShaperHeap,
) Push(
	x interface{},
) {
	*h = append(*h, x.(WriteRequest))
}

// Pop ...
func (
	h *ShaperHeap,
) Pop() interface{} {
	old := *h
	n := len(
		old,
	)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
