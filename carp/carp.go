/*
   Cache Array Routing Protocol v1.0
   http://icp.ircache.net/carp.txt

   But we changed the original Url and Proxy Member from string to hash
   values. This lets the user be able to choose their favorite hash functions.
   We also changed the hash combination function to get an uniform 
   distribution result.
*/
package carp

import (
	"sort"
	"math"
	"container/heap"
)


type CombineFunc func(member uint64, key uint32) uint32

type Carp interface {
	Count() int
	Which(key uint32) int
	Sequence(key uint32, seqs []int) []int
}


type _Item struct {
	hash uint64
	factor float64
}

type _Carp struct {
	weighted bool
	combine CombineFunc
	items []_Item
}


func newCarp(members []uint64, combine CombineFunc) *_Carp {
	if len(members) == 0 {
		panic("Slice members can't have length 0")
	}
	if combine == nil {
		combine = myCombine
	}

	items := make([]_Item, 0, len(members))
	for _, m := range members {
		items = append(items, _Item{m, 1})
	}
	cp := &_Carp{combine:combine, items:items}
	return cp
}

// NewCarp return a new Carp. If the combine function is nil, the default is used.
func NewCarp(members []uint64, combine CombineFunc) Carp {
	return newCarp(members, combine)
}

// NewCarp return a new Carp with weighted members.
// If the combine function is nil, the default is used.
func NewCarpWeight(members []uint64, weights []uint32, combine CombineFunc) Carp {
	cp := newCarp(members, combine)

	sum := 0.0
	weight := weights[0]
	for i := 0; i < len(cp.items); i++ {
		sum += float64(weights[i])
		if weights[i] != weight {
			cp.weighted = true
		}
	}

	if cp.weighted {
		ix := make([]int, len(members))
		for i := 0; i < len(ix); i++ {
			ix[i] = i
		}

		sort.Slice(ix, func(i, j int) bool {
			a := ix[i]
			b := ix[j]
			if (weights[a] < weights[b]) {
				return true
			} else if (weights[a] == weights[b] && a < b) {
				return true
			}
			return false
		})

		num := len(ix)
		i := 0
		for ; i < num; i++ {
			k := ix[i]
			if weights[k] > 0 {
				break
			}
			cp.items[k].factor = 0.0
		}

		k := ix[i]
                p0 := float64(weights[k]) / sum
                x0 := math.Pow(float64(num) * p0, 1.0 / float64(num))
                product := x0
                cp.items[k].factor = x0

		for i++; i < num; i++ {
                        p1 := p0
                        x1 := x0
                        ni := float64(num - i)

                        k := ix[i]
                        p0 = float64(weights[k]) / sum
                        x0 = ni * (p0 - p1) / product
                        x0 += math.Pow(x1, ni)
                        x0 = math.Pow(x0, 1.0 / ni)
                        product *= x0
                        cp.items[k].factor = x0
                }
	}
	return cp
}

// http://www.cris.com/~Ttwang/tech/inthash.htm
func hash32shiftmult(a uint32) uint32 {
        a = (a ^ 61) ^ (a >> 16)
        a = a + (a << 3)
        a = a ^ (a >> 4)
        a = a * 0x27d4eb2d
        a = a ^ (a >> 15)
        return a
}

func myCombine(m uint64, k uint32) uint32 {
        low := uint32(m)
	high := uint32(m >> 32)
        k = (k - low) ^ high;
        return hash32shiftmult(k)
}

// Count return number of members
func (cp *_Carp) Count() int {
	return len(cp.items)
}

// Which return the index of the choosed member which has the maxium return value of the combine function.
func (cp *_Carp) Which(key uint32) int {
	idx := 0
	if cp.weighted {
                max := 0.0
                for i := 0; i < len(cp.items); i++ {
			it := &cp.items[i]
                        x := it.factor * float64(cp.combine(it.hash, key))
                        if max < x {
                                max = x
                                idx = i
                        }
                }
	} else {
		max := uint32(0)
                for i := 0; i < len(cp.items); i++ {
			it := &cp.items[i]
                        x := cp.combine(it.hash, key)
                        if max < x {
                                max = x
                                idx = i
                        }
                }
	}
	return idx
}

type _ValueSeqs struct {
	n int
	items []_VsItem
}

type _VsItem struct {
	v float64
	s int
}

func (o *_ValueSeqs) Len() int {
	return o.n
}

func (o *_ValueSeqs) Less(i, j int) bool {
	a := &o.items[i]
	b := &o.items[j]
	return (a.v < b.v || (a.v == b.v && a.s < b.s))
}

func (o *_ValueSeqs) Swap(i, j int) {
	o.items[i], o.items[j] = o.items[j], o.items[i]
}

func (o *_ValueSeqs) Push(x any) {
	z := x.(_VsItem)
	o.items[o.n] = z
	o.n++
}

func (o *_ValueSeqs) Pop() any {
	o.n--
	return o.items[o.n]
}


// Sequence return the sequence of members' indexes with the maxium return values of the combine function.
func (cp *_Carp) Sequence(key uint32, seqs []int) []int {
	if len(seqs) == 0 {
		return seqs
	}
	if len(seqs) > len(cp.items) {
		seqs = seqs[:len(cp.items)]
	}

	vs := &_ValueSeqs{n:0, items:make([]_VsItem, len(seqs))}
	i := 0
	for ; i < len(seqs); i++ {
		it := &cp.items[i]
		x := it.factor * float64(cp.combine(it.hash, key))
		heap.Push(vs, _VsItem{x, i})
	}
	for ; i < len(cp.items); i++ {
		it := &cp.items[i]
		x := it.factor * float64(cp.combine(it.hash, key))
		if x > vs.items[0].v {
			vs.items[0].v = x
			vs.items[0].s = i
			heap.Fix(vs, 0)
		}
	}

	for i := len(seqs) - 1; i >= 0; i-- {
		z := heap.Pop(vs).(_VsItem)
		seqs[i] = z.s
	}

	return seqs
}

