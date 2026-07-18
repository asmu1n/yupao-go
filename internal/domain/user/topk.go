package user

import "container/heap"

type scoredUser struct {
	userID   int64
	distance int
}

type maxHeap []scoredUser

func (h maxHeap) Len() int {
	return len(h)
}
func (h maxHeap) Less(i, j int) bool {
	return h[i].distance > h[j].distance
}
func (h maxHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}
func (h *maxHeap) Push(x any) {
	*h = append(*h, x.(scoredUser))
}
func (h *maxHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func topKNearest(candidates []scoredUser, num int) []int64 {
	if num <= 0 {
		return nil
	}
	h := make(maxHeap, 0, num)
	for _, c := range candidates {
		if h.Len() < num {
			heap.Push(&h, c)
			continue
		}
		if c.distance < h[0].distance {
			heap.Pop(&h)
			heap.Push(&h, c)
		}
	}

	ids := make([]int64, h.Len())
	for i := len(ids) - 1; i >= 0; i-- {
		ids[i] = heap.Pop(&h).(scoredUser).userID
	}
	return ids
}
