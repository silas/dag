# dag

This is a slightly modified version of the Terraform [dag][dag] code with the Terraform dependencies removed.

## Example

``` go
package main

import (
	"fmt"
	"time"

	"github.com/silas/dag"
)

func main() {
	g := &dag.AcyclicGraph{}

	p := "<complete>"

	g.Add(p)

	g.Add("a")
	g.Connect(dag.BasicEdge(p, "a"))

	g.Add("b")
	g.Connect(dag.BasicEdge(p, "b"))
	g.Connect(dag.BasicEdge("b", "a"))

	g.Add("c")
	g.Connect(dag.BasicEdge(p, "c"))
	g.Connect(dag.BasicEdge("c", "a"))

	if err := g.Validate(); err != nil {
		panic(err)
	}

	g.Walk(func(v dag.Vertex) (d dag.Diagnostics) {
		key := v.(string)

		fmt.Println(key)
		time.Sleep(2 * time.Second)

		return
	})
}
```

[dag]: https://github.com/hashicorp/terraform/tree/main/internal/dag
