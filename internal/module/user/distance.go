package user

// minDistance 计算两组标签的最小编辑距离，用于匹配相似度（距离越小越相似）。
func minDistance(tags1, tags2 []string) int {
	n := len(tags1)
	m := len(tags2)

	if n*m == 0 {
		return n + m
	}

	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			left := d[i-1][j] + 1
			down := d[i][j-1] + 1
			leftDown := d[i-1][j-1]
			if tags1[i-1] != tags2[j-1] {
				leftDown++
			}
			d[i][j] = min(left, min(down, leftDown))
		}
	}
	return d[n][m]
}
