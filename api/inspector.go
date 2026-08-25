package api

// EngineInspector reports the operations an engine has registered.
//
// It is an optional interface rather than a method on Engine so that adding
// introspection does not break a host that implements Engine itself; callers
// type-assert for it, the way the framework's other inspection seams work.
type EngineInspector interface {
	// Operations returns every registered operation, ordered by identifier so
	// repeated calls and repeated runs produce the same sequence.
	Operations() []*Operation
}
