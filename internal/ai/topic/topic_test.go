package topic

import (
	"math"
	"testing"

	"GopherPaper/internal/model"
)

// buildTopics 用给定质心向量造一批 model.Topic,质心 JSON 编码,供 nearest 测试。
func buildTopics(centroids map[string][]float64) []model.Topic {
	topics := make([]model.Topic, 0, len(centroids))
	for id, c := range centroids {
		topics = append(topics, model.Topic{ID: id, Centroid: encode(c), MemberCount: 1})
	}
	return topics
}

const eps = 1e-9

func TestCosine(t *testing.T) {
	cases := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"相同方向", []float64{1, 0}, []float64{2, 0}, 1},
		{"正交", []float64{1, 0}, []float64{0, 1}, 0},
		{"反向", []float64{1, 0}, []float64{-1, 0}, -1},
		{"长度不一", []float64{1, 0}, []float64{1, 0, 0}, 0},
		{"零向量", []float64{0, 0}, []float64{1, 1}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cosine(c.a, c.b); math.Abs(got-c.want) > eps {
				t.Errorf("cosine(%v,%v)=%v want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	v := []float64{3, 4}
	normalize(v)
	if math.Abs(v[0]-0.6) > eps || math.Abs(v[1]-0.8) > eps {
		t.Fatalf("normalize 结果错误: %v", v)
	}
	zero := []float64{0, 0}
	normalize(zero) // 零向量不变,不应 panic 或产生 NaN
	if zero[0] != 0 || zero[1] != 0 {
		t.Fatalf("零向量不应被改动: %v", zero)
	}
}

func TestAddSubVecAreInverse(t *testing.T) {
	// 质心(成员向量之和)加入 vec 再扣减,应还原。
	sum := []float64{1, 2, 3}
	vec := []float64{0.5, -1, 0.25}
	added := addVec(sum, vec)
	back := subVec(added, vec)
	for i := range sum {
		if math.Abs(back[i]-sum[i]) > eps {
			t.Fatalf("add 后 sub 未还原: %v != %v", back, sum)
		}
	}
}

func TestAddSubVecFallback(t *testing.T) {
	// addVec 一方为空回退另一方;subVec 减空回退副本;长度不一返回 a 的副本。
	if got := addVec(nil, []float64{1, 2}); got[0] != 1 || got[1] != 2 {
		t.Fatalf("addVec(nil,b) 应回退 b: %v", got)
	}
	if got := subVec([]float64{3, 4}, nil); got[0] != 3 || got[1] != 4 {
		t.Fatalf("subVec(a,nil) 应回退 a: %v", got)
	}
	if got := addVec([]float64{1, 2}, []float64{1}); len(got) != 2 || got[0] != 1 {
		t.Fatalf("长度不一应返回 a 的副本: %v", got)
	}
}

func TestRefineReplacePreservesSum(t *testing.T) {
	// 同主题 refine:sum + new - old 把会话旧贡献换成新贡献,成员数不变。
	// 主题原由两个会话(1,0)、(0,1)组成,sum=(1,1);其中一个会话由(0,1)refine 到(0.6,0.8)。
	sum := []float64{1, 1}
	old := []float64{0, 1}
	neu := []float64{0.6, 0.8}
	got := addVec(subVec(sum, old), neu)
	want := []float64{1.6, 0.8} // (1,0)+(0.6,0.8)
	for i := range want {
		if math.Abs(got[i]-want[i]) > eps {
			t.Fatalf("refine 替换结果错误: %v want %v", got, want)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	v := []float64{0.1, -0.2, 0.3}
	got := decode(encode(v))
	if len(got) != len(v) {
		t.Fatalf("decode 长度不符: %v", got)
	}
	for i := range v {
		if math.Abs(got[i]-v[i]) > eps {
			t.Fatalf("encode/decode 往返失真: %v != %v", got, v)
		}
	}
	if decode("") != nil || decode("not-json") != nil {
		t.Fatalf("非法 JSON 应解码为 nil")
	}
}

func TestNearestPicksMostSimilar(t *testing.T) {
	topics := buildTopics(map[string][]float64{
		"a": {1, 0},
		"b": {0, 1},
	})
	best, sim := nearest(topics, []float64{0.9, 0.1})
	if best == nil || best.ID != "a" {
		t.Fatalf("nearest 应选 a, got %+v", best)
	}
	if sim <= 0.9 {
		t.Fatalf("相似度应较高, got %v", sim)
	}
	// 空主题列表返回 nil。
	if b, _ := nearest(nil, []float64{1, 0}); b != nil {
		t.Fatalf("空列表应返回 nil")
	}
}

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"  注意力机制 ":    "注意力机制",
		"「咖啡点单」":      "咖啡点单",
		"论文检索\n额外的解释": "论文检索",
		"综述:":         "综述",
	}
	for in, want := range cases {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q)=%q want %q", in, got, want)
		}
	}
}
