package graph

import "sort"

// weightedEdge 是关键词共现的一条带权无向边。
type weightedEdge struct {
	A string
	B string
	W int
}

// detectCommunities 用标签传播(Label Propagation)给每个节点分配社区号(压缩为 0..k-1)。
// 按节点 id 排序迭代、平票取标签字典序小者,保证确定性,无随机;孤立点自成一团。
func detectCommunities(nodeIDs []string, edges []weightedEdge) map[string]int {
	adj := make(map[string]map[string]int, len(nodeIDs))
	for _, id := range nodeIDs {
		adj[id] = map[string]int{}
	}
	for _, e := range edges {
		if e.A == e.B {
			continue
		}
		if _, ok := adj[e.A]; !ok {
			continue
		}
		if _, ok := adj[e.B]; !ok {
			continue
		}
		adj[e.A][e.B] += e.W
		adj[e.B][e.A] += e.W
	}

	order := make([]string, len(nodeIDs))
	copy(order, nodeIDs)
	sort.Strings(order)

	label := make(map[string]string, len(order))
	for _, id := range order {
		label[id] = id
	}

	const maxIter = 20
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for _, id := range order {
			neighbors := adj[id]
			if len(neighbors) == 0 {
				continue
			}
			score := map[string]int{}
			for nb, w := range neighbors {
				score[label[nb]] += w
			}
			keys := make([]string, 0, len(score))
			for k := range score {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			best := label[id]
			bestScore := -1
			for _, k := range keys {
				if score[k] > bestScore {
					bestScore = score[k]
					best = k
				}
			}
			if best != label[id] {
				label[id] = best
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	community := make(map[string]int, len(order))
	idx := map[string]int{}
	next := 0
	for _, id := range order {
		lb := label[id]
		if _, ok := idx[lb]; !ok {
			idx[lb] = next
			next++
		}
		community[id] = idx[lb]
	}
	return community
}
