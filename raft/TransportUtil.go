package raft

// AttachTransport registers a Node with the given Transport for inbound RPCs.
// No-op if the underlying implementation does not support transport wiring.
func AttachTransport(n Node, t Transport) {
	if impl, ok := n.(*nodeImpl); ok {
		impl.WithTransport(t)
	}
}
