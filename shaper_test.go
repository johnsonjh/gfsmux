package gfsmux_test

import (
	"container/heap"
	"testing"

	smux "go.gridfinity.dev/gfsmux"
	u "go.gridfinity.dev/leaktestfe"
)

func TestShaper(
	t *testing.T,
) {
	defer u.Leakplug(
		t,
	)
	w1 := smux.WriteRequest{Prio: 10}
	w2 := smux.WriteRequest{Prio: 10}
	w3 := smux.WriteRequest{Prio: 20}
	w4 := smux.WriteRequest{Prio: 100}

	var reqs smux.ShaperHeap
	heap.Push(
		&reqs,
		w4,
	)
	heap.Push(
		&reqs,
		w3,
	)
	heap.Push(
		&reqs,
		w2,
	)
	heap.Push(
		&reqs,
		w1,
	)

	var lastPrio uint64
	for len(
		reqs,
	) > 0 {
		w := heap.Pop(&reqs).(smux.WriteRequest)
		if w.Prio < lastPrio {
			t.Fatal(
				"incorrect shaper priority",
			)
		}

		t.Log(
			"Prio:",
			w.Prio,
		)
		lastPrio = w.Prio
	}
}
