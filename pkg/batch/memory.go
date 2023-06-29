package batch

import (
	"fmt"
	llq "github.com/emirpasic/gods/queues/linkedlistqueue"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

var (
	DefaultTaskWaitDuration int64 = 50
	DefaultQueueConsumeRate       = 100
	ErrorTimedOut                 = fmt.Errorf("task timed out in queue")
)

const (
	FlagAboutToClose = iota + 1
	FlagClosing
	FlagClosed
	ConsumerInterval = 10
)

// MemoryBatchQueue implements Queue. All tasks are stored in memory
type MemoryBatchQueue struct {
	q          *llq.Queue         // The underlying single linked queue, incoming requests are first stored in here
	mu         sync.Mutex         // Protects q
	bsp        BatchSizeProvider  // Batch size provider, provides batch size for specified partition
	hdl        QueueTaskHandler   // Queue task handler, user business
	pool       *ants.PoolWithFunc // Goroutine pool
	partitions sync.Map           // A map of partition and temporary task queue
	flag       int                // Queue flag indicates whether the queue is closing
	triggerC   chan struct{}      // Channel to trigger partition iteration
}

// NewMemoryBatchQueue initializes a MemoryBatchQueue. poolSize is the size of goroutine pool
func NewMemoryBatchQueue(bsp BatchSizeProvider, hdl QueueTaskHandler, poolSize int) *MemoryBatchQueue {
	q := &MemoryBatchQueue{
		q:        llq.New(),
		bsp:      bsp,
		hdl:      hdl,
		triggerC: make(chan struct{}),
	}
	fn := func(i interface{}) {
		tasks, ok := i.([]QueueTask)
		if !ok || len(tasks) == 0 {
			return
		}
		// Tasks are ensured that all tasks share the same partition name
		hdl(tasks[0].GetPartition(), tasks)
	}
	q.pool, _ = ants.NewPoolWithFunc(poolSize, fn, ants.WithExpiryDuration(time.Second*10))
	q.start()
	return q
}

// Push pushes QueueTasks in queue, returns a int64 channel to listen on, once all
// tasks are processed, a byte map number equals 1<<n-1 is sent to the channel.
// This byte map number can be used to find out which tasks are finished.
// When the queue is closing, or tasks is empty, a buffered channel is returned
// so that caller can go ahead without blocking.
func (q *MemoryBatchQueue) Push(tasks ...QueueTask) chan int64 {
	if q.flag > FlagAboutToClose || len(tasks) == 0 {
		// If the queue was closed, or tasks is empty, return a buffered
		// channel to avoid blocking on this call
		finishC := make(chan int64, 1)
		finishC <- 0
		return finishC
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	var (
		mu       sync.Mutex         // Mutex lock to protect the counter
		n        = len(tasks)       // Number of tasks
		finished = 0                // Finished task counter
		finishC  = make(chan int64) // Must be blocking
	)

	for i := range tasks {

		// Closure function to notify whether tasks are processed
		fn := func() {
			mu.Lock()
			finished++
			mu.Unlock()

			if finished == n /* All task finished */ {
				finishC <- int64(finished)
			}
		}
		tasks[i].WithFinishFunc(fn)
		q.q.Enqueue(tasks[i])
	}
	return finishC
}

func (q *MemoryBatchQueue) Close() error {
	if q.flag == 0 {
		q.flag = FlagAboutToClose
	}
	return nil
}

func (q *MemoryBatchQueue) Closed() bool {
	return q.flag == FlagClosed
}

// popN removes n QueueTasks from memory queue and returns them
func (q *MemoryBatchQueue) popN(n int) []QueueTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	tasks := make([]QueueTask, 0, n)
	for len(tasks) < n {
		v, ok := q.q.Dequeue()
		if !ok {
			break
		}
		tasks = append(tasks, v.(QueueTask))
	}
	return tasks
}

// partitionBatchSize returns partition batch size for given partition name
func (q *MemoryBatchQueue) partitionBatchSize(v string) int {
	return q.bsp.Get(v)
}

// pushPartitionQueue pushes a QueueTask into temporary partition queue
func (q *MemoryBatchQueue) pushPartitionQueue(v QueueTask) {
	s, ok := q.partitions.Load(v.GetPartition())
	if ok {
		s.(*partitionQueue).push(v)
		return
	}

	s = newPartitionQueue()
	act, ok1 := q.partitions.LoadOrStore(v.GetPartition(), s)
	if ok1 {
		act.(*partitionQueue).push(v)
	} else {
		s.(*partitionQueue).push(v)
	}
}

// process splits multiple QueueTasks into batches, invoke them within the goroutine pool
func (q *MemoryBatchQueue) process(tasks []QueueTask, batchSize int) {
	l := len(tasks)
	ns := l / batchSize
	if ns*batchSize < l {
		ns += 1
	}

	for i := 0; i < ns; i++ {
		lo := batchSize * i
		hi := batchSize * (i + 1)
		if l < hi {
			hi = l
		}
		tmp := tasks[lo:hi]
		if err := q.pool.Invoke(tmp); err != nil {
			log.Err(err).Msg("failed to invoke pool function")
			for _, t := range tmp {
				t.SetError(err)
			}
		}
		log.Trace().Int("batch", i).Int("batchSize", len(tmp)).Int("total", l).
			Msg("processed batch")
	}
}

func (q *MemoryBatchQueue) iteratePartitions() {
	q.partitions.Range(func(k, v interface{}) bool {
		size := q.partitionBatchSize(k.(string))
		tasks := v.(*partitionQueue).maybePop(size)
		if len(tasks) != 0 {
			log.Trace().Str("module", "BatchQueue").Int("tasks", len(tasks)).
				Str("partition", k.(string)).Msg("popped tasks")
			q.process(tasks, size)
		}
		return true
	})
}

// start starts 2 goroutines in background, one pulls from memory and pushes tasks into partitionQueue,
// while the other one loops over all partitions, fetch task batches and process them
func (q *MemoryBatchQueue) start() {
	// task partition consumer
	go func() {
		ticker := time.NewTicker(time.Millisecond * ConsumerInterval)

		for {
			select {
			case <-ticker.C:
				{
					// log.Trace().Msg("by ticker.C")
					q.iteratePartitions()
				}
			case <-q.triggerC:
				{
					// log.Trace().Msg("by triggerC")
					ticker.Reset(time.Millisecond * ConsumerInterval)
					q.iteratePartitions()
				}
			}
			// Double check to avoid leaking tasks
			if q.flag == FlagClosed {
				break
			}
			if q.flag == FlagClosing {
				// Will exit the for loop after next iteration
				q.flag = FlagClosed
			}
		}
	}()

	// task partition producer
	go func() {
		for {
			if q.flag == FlagClosing {
				break
			}
			tasks := q.popN(DefaultQueueConsumeRate)
			if q.flag == FlagAboutToClose && len(tasks) == 0 {
				q.flag = FlagClosing
			}

			for i, t := range tasks {
				if t.IsTimeout() {
					t.SetError(ErrorTimedOut)
				} else {
					q.pushPartitionQueue(tasks[i])
				}
			}

			if len(tasks) > 0 {
				q.triggerC <- struct{}{}
			}
			time.Sleep(time.Millisecond * 10)
		}
	}()
}

// partitionQueue is a temporary queue for partition, does not hold tasks too long
type partitionQueue struct {
	mu          sync.Mutex
	q           *llq.Queue
	firstQueued int64 // Timestamp in milliseconds when the first task is pushed into the partitionQueue
}

func newPartitionQueue() *partitionQueue {
	return &partitionQueue{
		q: llq.New(),
	}
}

// push pushes a QueueTask into partitionQueue
func (pq *partitionQueue) push(v QueueTask) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.q.Enqueue(v)
	pq.maybeFirstTaskQueued()
}

// reset clears queue data and resets first task queued timestamp
func (pq *partitionQueue) reset() {
	pq.q.Clear()
	pq.firstQueued = 0
}

// tasks returns all tasks stored in partitionQueue at this moment
func (pq *partitionQueue) tasks() []QueueTask {
	vals := pq.q.Values()
	tasks := make([]QueueTask, 0, len(vals))
	for i := range vals {
		tasks = append(tasks, vals[i].(QueueTask))
	}
	return tasks
}

// maybePop checks whether there are enough tasks to form a batch, or first queued is ready to go.
// When condition is met, returns tasks and reset itself
func (pq *partitionQueue) maybePop(n int) []QueueTask {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.q.Size() >= n || pq.isFirstTaskReady() {
		tasks := pq.tasks()
		pq.reset()
		return tasks
	}
	return nil
}

// isFirstTaskReady compares the firstQueued with current timestamp
func (pq *partitionQueue) isFirstTaskReady() bool {
	return time.Now().UnixNano()/1e6-pq.firstQueued > DefaultTaskWaitDuration
}

// maybeFirstTaskQueued when first task is queued (after creation or reset), updates the firstQueued timestamp
func (pq *partitionQueue) maybeFirstTaskQueued() {
	if pq.firstQueued == 0 {
		pq.firstQueued = time.Now().UnixNano() / 1e6
	}
}

// isEmpty returns whether partition queue is empty
func (pq *partitionQueue) isEmpty() bool {
	return pq.q.Empty()
}
