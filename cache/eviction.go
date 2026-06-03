package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// EvictionPolicy defines the eviction strategy for cache when it reaches max size.
type EvictionPolicy int

const (
	// EvictionPolicyNone disables eviction tracking (used for unlimited caches).
	EvictionPolicyNone EvictionPolicy = iota
	// EvictionPolicyLRU evicts least recently used entries when cache is full.
	EvictionPolicyLRU
	// EvictionPolicyLFU evicts least frequently used entries when cache is full.
	EvictionPolicyLFU
	// EvictionPolicyFIFO evicts oldest entries when cache is full.
	EvictionPolicyFIFO
)

// evictionPolicyHandler tracks access/insert order so the memory cache can pick
// a victim when it is full.
type evictionPolicyHandler interface {
	// OnAccess records that an entry was read.
	OnAccess(key string)
	// OnInsert records that a new entry was stored.
	OnInsert(key string)
	// OnEvict drops an entry from the tracking state.
	OnEvict(key string)
	// SelectEvictionCandidate selects a key to evict, returns empty string if no candidate.
	SelectEvictionCandidate() string
	// Reset clears all tracking state.
	Reset()
}

// newEvictionHandler builds the handler matching the requested policy.
func newEvictionHandler(policy EvictionPolicy) evictionPolicyHandler {
	switch policy {
	case EvictionPolicyLRU:
		return newLruHandler()
	case EvictionPolicyLFU:
		return newLfuHandler()
	case EvictionPolicyFIFO:
		return newFifoHandler()
	case EvictionPolicyNone:
		fallthrough
	default:
		return new(noOpEvictionHandler)
	}
}

// noOpEvictionHandler is used when eviction policy is EvictionPolicyNone.
type noOpEvictionHandler struct{}

func (*noOpEvictionHandler) OnAccess(string)                 {}
func (*noOpEvictionHandler) OnInsert(string)                 {}
func (*noOpEvictionHandler) OnEvict(string)                  {}
func (*noOpEvictionHandler) SelectEvictionCandidate() string { return "" }
func (*noOpEvictionHandler) Reset()                          {}

// lruHandler implements Least Recently Used eviction policy.
type lruHandler struct {
	mu         sync.RWMutex
	accessList *list.List
	accessMap  map[string]*list.Element
}

func newLruHandler() *lruHandler {
	return &lruHandler{
		accessList: list.New(),
		accessMap:  make(map[string]*list.Element),
	}
}

func (h *lruHandler) OnAccess(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if elem, exists := h.accessMap[key]; exists {
		// Move to front (most recently used)
		h.accessList.MoveToFront(elem)
	} else {
		// Add to front
		elem := h.accessList.PushFront(key)
		h.accessMap[key] = elem
	}
}

func (h *lruHandler) OnInsert(key string) {
	// Treat insert as access
	h.OnAccess(key)
}

func (h *lruHandler) OnEvict(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if elem, exists := h.accessMap[key]; exists {
		h.accessList.Remove(elem)
		delete(h.accessMap, key)
	}
}

func (h *lruHandler) SelectEvictionCandidate() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return least recently used (back of list)
	elem := h.accessList.Back()
	if elem == nil {
		return ""
	}

	if key, ok := elem.Value.(string); ok {
		return key
	}

	return ""
}

func (h *lruHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.accessList = list.New()
	h.accessMap = make(map[string]*list.Element)
}

// fifoHandler implements First In First Out eviction policy.
type fifoHandler struct {
	mu         sync.RWMutex
	insertList *list.List
	insertMap  map[string]*list.Element
}

func newFifoHandler() *fifoHandler {
	return &fifoHandler{
		insertList: list.New(),
		insertMap:  make(map[string]*list.Element),
	}
}

func (*fifoHandler) OnAccess(string) {
	// FIFO doesn't track access, only insertion order
}

func (h *fifoHandler) OnInsert(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.insertMap[key]; !exists {
		// Add to back (newest entries)
		elem := h.insertList.PushBack(key)
		h.insertMap[key] = elem
	}
}

func (h *fifoHandler) OnEvict(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if elem, exists := h.insertMap[key]; exists {
		h.insertList.Remove(elem)
		delete(h.insertMap, key)
	}
}

func (h *fifoHandler) SelectEvictionCandidate() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return oldest (front of list)
	elem := h.insertList.Front()
	if elem == nil {
		return ""
	}

	if key, ok := elem.Value.(string); ok {
		return key
	}

	return ""
}

func (h *fifoHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.insertList = list.New()
	h.insertMap = make(map[string]*list.Element)
}

// lfuNode represents a node in the LFU frequency list.
type lfuNode struct {
	key         string
	frequency   int64
	insertOrder int64 // For tie-breaking
}

// lfuFreqBucket represents a bucket of entries with the same frequency.
type lfuFreqBucket struct {
	frequency int64
	entries   *list.List // List of *lfuNode
	nodeMap   map[string]*list.Element
}

// lfuHandler implements Least Frequently Used eviction policy using frequency buckets.
// This implementation achieves O(1) time complexity for all operations.
type lfuHandler struct {
	mu            sync.RWMutex
	freqBuckets   *list.List // List of *lfuFreqBucket, sorted by frequency
	bucketMap     map[int64]*list.Element
	keyToBucket   map[string]*list.Element // Maps key to its bucket element
	keyToNode     map[string]*lfuNode
	minFreq       int64
	insertCounter int64
}

func newLfuHandler() *lfuHandler {
	return &lfuHandler{
		freqBuckets: list.New(),
		bucketMap:   make(map[int64]*list.Element),
		keyToBucket: make(map[string]*list.Element),
		keyToNode:   make(map[string]*lfuNode),
		minFreq:     0,
	}
}

func (h *lfuHandler) OnAccess(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, exists := h.keyToNode[key]
	if !exists {
		return
	}

	// Increment frequency
	oldFreq := node.frequency
	newFreq := oldFreq + 1
	node.frequency = newFreq

	// Move node to new frequency bucket. moveToFreqBucket deletes the old
	// bucket from bucketMap/freqBuckets once it empties, so recompute minFreq
	// from the surviving buckets rather than probing the (possibly removed)
	// old bucket.
	h.moveToFreqBucket(key, node, oldFreq, newFreq)

	if oldFreq == h.minFreq {
		h.recalculateMinFreq()
	}
}

func (h *lfuHandler) OnInsert(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.keyToNode[key]; exists {
		return
	}

	// Create new node with frequency 1
	insertOrder := atomic.AddInt64(&h.insertCounter, 1)
	node := &lfuNode{
		key:         key,
		frequency:   1,
		insertOrder: insertOrder,
	}
	h.keyToNode[key] = node

	// Add to frequency 1 bucket
	h.addToFreqBucket(key, node, 1)

	// Set minimum frequency
	h.minFreq = 1
}

func (h *lfuHandler) OnEvict(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, exists := h.keyToNode[key]
	if !exists {
		return
	}

	// Remove from frequency bucket
	h.removeFromFreqBucket(key, node.frequency)

	// Clean up
	delete(h.keyToNode, key)
	delete(h.keyToBucket, key)

	// Recalculate min frequency if needed
	if node.frequency == h.minFreq {
		h.recalculateMinFreq()
	}
}

func (h *lfuHandler) SelectEvictionCandidate() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.keyToNode) == 0 {
		return ""
	}

	// Get the minimum frequency bucket
	bucketElem, exists := h.bucketMap[h.minFreq]
	if !exists || bucketElem == nil {
		return ""
	}

	bucket, ok := bucketElem.Value.(*lfuFreqBucket)
	if !ok || bucket.entries.Len() == 0 {
		return ""
	}

	// Return the first entry (oldest by insertion order due to FIFO within bucket)
	elem := bucket.entries.Front()
	if elem == nil {
		return ""
	}

	if node, ok := elem.Value.(*lfuNode); ok {
		return node.key
	}

	return ""
}

func (h *lfuHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.freqBuckets = list.New()
	h.bucketMap = make(map[int64]*list.Element)
	h.keyToBucket = make(map[string]*list.Element)
	h.keyToNode = make(map[string]*lfuNode)
	h.minFreq = 0
	h.insertCounter = 0
}

// addToFreqBucket adds a node to the specified frequency bucket.
func (h *lfuHandler) addToFreqBucket(key string, node *lfuNode, freq int64) {
	bucketElem, exists := h.bucketMap[freq]

	var bucket *lfuFreqBucket

	if !exists {
		// Create new bucket
		bucket = &lfuFreqBucket{
			frequency: freq,
			entries:   list.New(),
			nodeMap:   make(map[string]*list.Element),
		}

		// Insert bucket in sorted order
		bucketElem = h.insertBucketSorted(bucket)
		h.bucketMap[freq] = bucketElem
	} else if b, ok := bucketElem.Value.(*lfuFreqBucket); ok {
		bucket = b
	}

	// Add node to bucket (at back for FIFO within same frequency)
	nodeElem := bucket.entries.PushBack(node)
	bucket.nodeMap[key] = nodeElem
	h.keyToBucket[key] = bucketElem
}

// removeFromFreqBucket removes a node from the specified frequency bucket.
func (h *lfuHandler) removeFromFreqBucket(key string, freq int64) {
	bucketElem, exists := h.bucketMap[freq]
	if !exists {
		return
	}

	bucket, ok := bucketElem.Value.(*lfuFreqBucket)
	if !ok {
		return
	}

	nodeElem, exists := bucket.nodeMap[key]
	if !exists {
		return
	}

	// Remove node from bucket
	bucket.entries.Remove(nodeElem)
	delete(bucket.nodeMap, key)

	// If bucket is empty, remove it
	if bucket.entries.Len() == 0 {
		h.freqBuckets.Remove(bucketElem)
		delete(h.bucketMap, freq)
	}
}

// moveToFreqBucket moves a node from one frequency bucket to another.
func (h *lfuHandler) moveToFreqBucket(key string, node *lfuNode, oldFreq, newFreq int64) {
	h.removeFromFreqBucket(key, oldFreq)
	h.addToFreqBucket(key, node, newFreq)
}

// insertBucketSorted inserts a bucket in sorted order by frequency.
func (h *lfuHandler) insertBucketSorted(bucket *lfuFreqBucket) *list.Element {
	// Find insertion point
	for elem := h.freqBuckets.Front(); elem != nil; elem = elem.Next() {
		existingBucket, ok := elem.Value.(*lfuFreqBucket)
		if ok && bucket.frequency < existingBucket.frequency {
			return h.freqBuckets.InsertBefore(bucket, elem)
		}
	}

	// Insert at end
	return h.freqBuckets.PushBack(bucket)
}

// recalculateMinFreq recalculates the minimum frequency from current buckets.
func (h *lfuHandler) recalculateMinFreq() {
	if h.freqBuckets.Len() == 0 {
		h.minFreq = 0

		return
	}

	// The first bucket has the minimum frequency
	elem := h.freqBuckets.Front()
	if elem != nil {
		if bucket, ok := elem.Value.(*lfuFreqBucket); ok {
			h.minFreq = bucket.frequency
		}
	}
}
