package txmemory

// Name is the stable identifier used in routing configuration.
//
// tx_memory is the in-process counterpart of the transactional outbox:
// it satisfies the "events become visible iff the business transaction
// commits" contract that the approval and storage modules assert at
// start-up, but delivers in the publishing process instead of persisting
// a row for a relay to claim. Nothing is written to the database, so
// nothing survives a crash between commit and delivery.
//
// The intended use is development against a database shared by several
// running instances. There, the outbox relay is a lottery — any node's
// relay may claim any node's rows — so the handler for an action tends to
// run on a colleague's machine rather than the one being debugged.
// Routing to tx_memory keeps every event in the process that published
// it, and drops delivery latency from the relay poll interval to zero.
//
// It is deliberately a separate transport rather than a transactional
// mode of the memory transport: memory must keep declaring
// Transactional=false so that a production deployment which forgot to
// enable the outbox still fails fast instead of silently accepting an
// in-process, non-durable route for events that must not be lost.
const Name = "tx_memory"
