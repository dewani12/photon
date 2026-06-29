package metrics

import (
	"fmt"
	"sync/atomic"
)

type Counter struct{
	name string
	value atomic.Uint64 //prevent mutex overhead due to concurrency
	labels map[string]string //cardinality explosion
	help string
}

func NewCounter(name, help string, labels map[string]string) *Counter{
	c:=&Counter{
		name:name,
		help:help,
		labels:labels,
	}
	return c
}

func (c *Counter)Inc(){
	c.value.Add(1)
}

func (c *Counter)Add(n uint64){
	c.value.Add(n)
}

func (c *Counter)Value()uint64{
	return c.value.Load()
}

func (c *Counter)Render() string{
	return fmt.Sprintf(
        "# HELP %s %s\n# TYPE %s counter\n%s%s %d\n",
        c.name, c.help,
        c.name,
        c.name, renderLabels(c.labels),
        c.value.Load(),
    )
}

