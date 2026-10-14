package colorconvert

// route.js: routes a model to every other model through the direct
// conversions, by a breadth-first search over the conversion graph.

type graphNode struct {
	distance int
	parent   string // "" is null
}

// adjacency holds each model's direct conversions in Object.keys order.
var adjacency = func() map[string][]conversion {
	m := map[string][]conversion{}
	for _, c := range conversions {
		m[c.from] = append(m[c.from], c)
	}
	return m
}()

func direct(from, to string) conversion {
	for _, c := range adjacency[from] {
		if c.to == to {
			return c
		}
	}
	panic("colorconvert: no conversion " + from + " -> " + to)
}

// deriveBFS is route.js's deriveBFS. Its queue pops from the end and
// unshifts at the front, so it is first in, first out.
func deriveBFS(fromModel string) map[string]*graphNode {
	graph := make(map[string]*graphNode, len(Models))
	for _, m := range Models {
		graph[m] = &graphNode{distance: -1}
	}
	queue := []string{fromModel}
	graph[fromModel].distance = 0
	for len(queue) > 0 {
		current := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, c := range adjacency[current] {
			node := graph[c.to]
			if node.distance == -1 {
				node.distance = graph[current].distance + 1
				node.parent = current
				queue = append([]string{c.to}, queue...)
			}
		}
	}
	return graph
}

func link(from, to rawFunc) rawFunc {
	return func(args Value) Value { return to(from(args)) }
}

// routed is one entry of route(fromModel): the composed function and its
// path. A direct conversion is the conversions.js function itself, and
// route.js sets .conversion on that shared function object; id identifies it.
type routed struct {
	fn   rawFunc
	path []string
	id   string // "" for composed (link) functions
}

func wrapConversion(toModel string, graph map[string]*graphNode) routed {
	parent := graph[toModel].parent
	path := []string{parent, toModel}
	c := direct(parent, toModel)
	r := routed{fn: c.fn, id: c.from + "." + c.to}
	if c.id != "" {
		r.id = c.id
	}
	cur := parent
	for graph[cur].parent != "" {
		path = append([]string{graph[cur].parent}, path...)
		r.fn = link(direct(graph[cur].parent, cur).fn, r.fn)
		r.id = ""
		cur = graph[cur].parent
	}
	r.path = path
	return r
}

// route is route.js's export: every model reachable from fromModel, mapped
// to its conversion. The returned paths are the functions' .conversion
// properties as index.js reads them once route returns: convert.gray.hsl and
// convert.gray.hsv share one function object, so both report the path
// assigned last, ['gray', 'hsv'].
func route(fromModel string) map[string]routed {
	graph := deriveBFS(fromModel)
	conversion := map[string]routed{}
	lastPath := map[string][]string{} // .conversion of each shared function object
	for _, toModel := range Models {
		if graph[toModel].parent == "" {
			continue // no possible conversion, or the source model
		}
		r := wrapConversion(toModel, graph)
		if r.id != "" {
			lastPath[r.id] = r.path
		}
		conversion[toModel] = r
	}
	for to, r := range conversion {
		if r.id != "" {
			r.path = lastPath[r.id]
			conversion[to] = r
		}
	}
	return conversion
}
