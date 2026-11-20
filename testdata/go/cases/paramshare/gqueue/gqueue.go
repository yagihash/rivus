// Package gqueue: handler sends what it received on a global channel (a
// queue); worker receives it into a sink: 1 finding.
package gqueue

import "os"

type Req struct{ Path string }

var queue = make(chan string, 1)

func handler(r *Req) { queue <- r.Path }
func worker()        { os.Create(<-queue) }

func Main() {
	handler(&Req{})
	worker()
}
