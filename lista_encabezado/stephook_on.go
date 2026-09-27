//go:build stephook

package list_head

import "sync/atomic"

// StepHook is called at named points of insertion and deletion when the
// package is built with the stephook tag, so that a test can stop a goroutine
// there and replay a concurrent interleaving one step at a time.
//
// Points and arguments:
//   - "insert.begin" (new, nil, next): InsertBefore is called in
//     MODE_CONCURRENT and next is not marked.
//   - "add.cas1" (new, prev, next): before prev.next is changed to new.
//   - "add.cas2" (new, prev, next): prev.next is new; before next.prev is
//     changed.
//   - "add.rollback" (new, prev, next): next.prev was not changed; before
//     prev.next is put back.
//   - "del.purgeable" (node, nil, nil): canPurge of MarkForDelete has read the
//     links of node and found them linking to other nodes; before the
//     neighbors of node are read.
//   - "del.begin" (node, prev, next): MarkForDelete has read the neighbors of
//     node and marked neither link.
//   - "del.nextMarked" (node, prev, next): the next link of node is marked,
//     and the prev link is not.
//   - "del.marked" (node, prev, next): both links of node are marked.
//   - "del.relinkNext" (node, nil, next): a link to node from before it is
//     to be changed to next, chosen after checking the mark of the next node
//     of node; before the CAS.
//   - "del.check" (node, prev, next): the links to node are changed; before
//     they are checked.
//   - "prev.waitNoMark" (node, nil, nil): Prev in the WaitNoMark mode, with
//     DefaultModeTraverse already switched by the options of the call; before
//     node.prev is read.
//   - "safety.nodes" (node, prev, next): IsSafety of node has taken prev and
//     next, the nodes whose links it checks; before it checks them.
//   - "cursor.next" (pos, nil, nil): Cursor.Next is called with the cursor at
//     pos; before pos.next is read.
type StepHook func(point string, a, b, c *ListHead)

// stepHook holds a StepHook. atomic.Pointer is not used because this module
// is at go 1.17, before type parameters.
var stepHook atomic.Value

// SetStepHook installs fn, or removes the hook when fn is nil.
func SetStepHook(fn StepHook) {
	stepHook.Store(fn)
}

func stepAt(point string, a, b, c *ListHead) {
	if fn, _ := stepHook.Load().(StepHook); fn != nil {
		fn(point, a, b, c)
	}
}
