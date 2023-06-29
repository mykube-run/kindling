package batch

// Queue consistently accepts and buffers incoming tasks in underlying queue,
// reorganizes buffered tasks in batches, finally processes task batches in parallel.
// There are 2 circumstances in which tasks are processed - when the number of buffered
// tasks is greater than batch size, or the very first queued task has been waiting for a
// time longer than DefaultTaskWaitDuration.
// The size of a batch is provided by BatchSizeProvider and queued tasks are partitioned
// accordingly, e.g. by model id.
type Queue interface {
	// Push pushes QueueTasks in queue, returns a int64 channel to listen on. Once all
	// tasks are processed, a byte map number equals 1<<n-1 is sent to the channel.
	// This byte map number can be used to find out which tasks are finished.
	// When the queue is closing, or tasks is empty, a buffered channel is returned
	// so that listener can go ahead without blocking.
	Push(...QueueTask) chan int64
	Close() error
	Closed() bool
}

// QueueTask defines required methods for a request that can be processed by Queue.
// The QueueTask must be able to know when all tasks that are originally queued together are finished,
// e.g. keeping a finished task counter.
// See TestQueueTask.SetResult in tests for sample usage.
type QueueTask interface {
	// GetPartition returns QueueTask's partition name
	GetPartition() string

	// GetPayload returns QueueTask's payload, can be used for debugging purpose
	GetPayload() interface{}

	// IsTimeout determines whether the QueueTask has timed out
	IsTimeout() bool

	// SetResult sets task execution result back
	SetResult(interface{})

	// SetError sets task execution error
	SetError(error)

	// WithFinishFunc sets a callback for the task, must be called after SetResult or SetError
	WithFinishFunc(func())

	// GetResult returns task result, should return nil when no result can be found
	GetResult() interface{}

	// GetError returns task error
	GetError() error
}

// QueueTasks is an array of QueueTasks, this provides several convenient methods
type QueueTasks []QueueTask

// Payload returns QueueTasks' payload as an array
func (qt QueueTasks) Payload() []interface{} {
	if qt == nil {
		return nil
	}
	ret := make([]interface{}, 0, len(qt))
	for i, _ := range qt {
		ret = append(ret, qt[i].GetPayload())
	}
	return ret
}

// BatchSizeProvider provides a reasonable batch size for queued tasks for specified partition name
type BatchSizeProvider interface {
	Get(string) int
	Set(string, int)
}

// QueueTaskHandler is used to handle a types.QueueTask batch. This is often where user logic should be placed.
type QueueTaskHandler func(string, []QueueTask)
