package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSort_EmptyGraph(t *testing.T) {
	g := New()
	order, err := g.Sort()
	require.NoError(t, err)
	assert.Empty(t, order)
}

func TestSort_SingleNode(t *testing.T) {
	g := New()
	g.Add(Node{URI: "github.com/SomeOrg/nginx@v1.0.0"})

	order, err := g.Sort()
	require.NoError(t, err)
	assert.Equal(t, []string{"github.com/SomeOrg/nginx@v1.0.0"}, order)
}

func TestSort_LinearChain(t *testing.T) {
	// C requires B, B requires A → execution order: A, B, C
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C", Requires: []string{"B"}})

	order, err := g.Sort()
	require.NoError(t, err)

	idxA := indexOf(order, "A")
	idxB := indexOf(order, "B")
	idxC := indexOf(order, "C")

	assert.True(t, idxA < idxB, "A must come before B")
	assert.True(t, idxB < idxC, "B must come before C")
}

func TestSort_DiamondDependency(t *testing.T) {
	// D requires B and C, B requires A, C requires A → A first, D last
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C", Requires: []string{"A"}})
	g.Add(Node{URI: "D", Requires: []string{"B", "C"}})

	order, err := g.Sort()
	require.NoError(t, err)
	assert.Len(t, order, 4)

	idxA := indexOf(order, "A")
	idxB := indexOf(order, "B")
	idxC := indexOf(order, "C")
	idxD := indexOf(order, "D")

	assert.True(t, idxA < idxB, "A must come before B")
	assert.True(t, idxA < idxC, "A must come before C")
	assert.True(t, idxB < idxD, "B must come before D")
	assert.True(t, idxC < idxD, "C must come before D")
}

func TestSort_IndependentBranches(t *testing.T) {
	// Two independent chains: A→B and C→D (no ordering between branches)
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C"})
	g.Add(Node{URI: "D", Requires: []string{"C"}})

	order, err := g.Sort()
	require.NoError(t, err)
	assert.Len(t, order, 4)

	assert.True(t, indexOf(order, "A") < indexOf(order, "B"))
	assert.True(t, indexOf(order, "C") < indexOf(order, "D"))
}

func TestSort_CircularDependency(t *testing.T) {
	g := New()
	g.Add(Node{URI: "A", Requires: []string{"B"}})
	g.Add(Node{URI: "B", Requires: []string{"A"}})

	_, err := g.Sort()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular _requires reference")
}

func TestSort_SelfReference(t *testing.T) {
	g := New()
	g.Add(Node{URI: "A", Requires: []string{"A"}})

	_, err := g.Sort()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular _requires reference")
}

func TestSort_ThreeNodeCycle(t *testing.T) {
	g := New()
	g.Add(Node{URI: "A", Requires: []string{"C"}})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C", Requires: []string{"B"}})

	_, err := g.Sort()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular _requires reference")
}

func TestSort_MissingDependency(t *testing.T) {
	// B requires A, but A is not in the graph — should still work (A treated as leaf)
	g := New()
	g.Add(Node{URI: "B", Requires: []string{"A"}})

	order, err := g.Sort()
	require.NoError(t, err)
	// B should be in the output; A is referenced but not a node
	assert.Contains(t, order, "B")
}

func TestSort_RealisticPipeline(t *testing.T) {
	// Simulates: nginx requires postgres, postgres requires secrets, secrets requires nothing
	g := New()
	g.Add(Node{
		URI: "github.com/MyOrg/secrets@v1.0.0",
	})
	g.Add(Node{
		URI:      "github.com/MyOrg/postgres@v15.0.0",
		Requires: []string{"github.com/MyOrg/secrets@v1.0.0"},
	})
	g.Add(Node{
		URI:      "github.com/MyOrg/nginx@v1.24.0",
		Requires: []string{"github.com/MyOrg/postgres@v15.0.0"},
	})

	order, err := g.Sort()
	require.NoError(t, err)
	assert.Len(t, order, 3)

	idxSecrets := indexOf(order, "github.com/MyOrg/secrets@v1.0.0")
	idxPostgres := indexOf(order, "github.com/MyOrg/postgres@v15.0.0")
	idxNginx := indexOf(order, "github.com/MyOrg/nginx@v1.24.0")

	assert.True(t, idxSecrets < idxPostgres, "secrets must come before postgres")
	assert.True(t, idxPostgres < idxNginx, "postgres must come before nginx")
}

func TestLevels_EmptyGraph(t *testing.T) {
	g := New()
	levels, err := g.Levels()
	require.NoError(t, err)
	assert.Empty(t, levels)
}

func TestLevels_SingleNode(t *testing.T) {
	g := New()
	g.Add(Node{URI: "A"})

	levels, err := g.Levels()
	require.NoError(t, err)
	assert.Len(t, levels, 1)
	assert.Equal(t, []string{"A"}, levels[0])
}

func TestLevels_LinearChain(t *testing.T) {
	// A → B → C: three levels, one node each
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C", Requires: []string{"B"}})

	levels, err := g.Levels()
	require.NoError(t, err)
	assert.Len(t, levels, 3)
	assert.Contains(t, levels[0], "A")
	assert.Contains(t, levels[1], "B")
	assert.Contains(t, levels[2], "C")
}

func TestLevels_IndependentNodes(t *testing.T) {
	// A, B, C have no deps — all in one level (parallel)
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B"})
	g.Add(Node{URI: "C"})

	levels, err := g.Levels()
	require.NoError(t, err)
	assert.Len(t, levels, 1)
	assert.Len(t, levels[0], 3)
}

func TestLevels_Diamond(t *testing.T) {
	// A → B, A → C, B+C → D: levels = [A], [B,C], [D]
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C", Requires: []string{"A"}})
	g.Add(Node{URI: "D", Requires: []string{"B", "C"}})

	levels, err := g.Levels()
	require.NoError(t, err)
	assert.Len(t, levels, 3)
	assert.Equal(t, []string{"A"}, levels[0])
	assert.Len(t, levels[1], 2)
	assert.Contains(t, levels[1], "B")
	assert.Contains(t, levels[1], "C")
	assert.Equal(t, []string{"D"}, levels[2])
}

func TestLevels_TwoIndependentChains(t *testing.T) {
	// A→B and C→D: levels = [A,C], [B,D]
	g := New()
	g.Add(Node{URI: "A"})
	g.Add(Node{URI: "B", Requires: []string{"A"}})
	g.Add(Node{URI: "C"})
	g.Add(Node{URI: "D", Requires: []string{"C"}})

	levels, err := g.Levels()
	require.NoError(t, err)
	assert.Len(t, levels, 2)
	assert.Len(t, levels[0], 2)
	assert.Contains(t, levels[0], "A")
	assert.Contains(t, levels[0], "C")
	assert.Len(t, levels[1], 2)
	assert.Contains(t, levels[1], "B")
	assert.Contains(t, levels[1], "D")
}

func TestLevels_CircularDependency(t *testing.T) {
	g := New()
	g.Add(Node{URI: "A", Requires: []string{"B"}})
	g.Add(Node{URI: "B", Requires: []string{"A"}})

	_, err := g.Levels()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular _requires reference")
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
