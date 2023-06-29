package batch

import (
	"fmt"
	"github.com/rs/zerolog"
	"math/rand"
	"testing"
	"time"
)

type TestBatchSizeProvider struct {
}

func (bsp *TestBatchSizeProvider) Get(p string) int {
	return 8
}

func (bsp *TestBatchSizeProvider) Set(p string, n int) {
	return
}

type TestQueueTask struct {
	Index     int
	Partition string
	Payload   string
	Until     time.Time
	Result    string
	Error     error
	onFinish  func()
}

func (t *TestQueueTask) GetPartition() string {
	return t.Partition
}

func (t *TestQueueTask) GetPayload() interface{} {
	return t.Payload
}

func (t *TestQueueTask) IsTimeout() bool {
	return time.Now().After(t.Until)
}

func (t *TestQueueTask) SetResult(i interface{}) {
	t.Result = i.(string)
	t.onFinish()
	fmt.Println(fmt.Sprintf("set result: %v", t.Index))
}

func (t *TestQueueTask) SetError(err error) {
	t.Error = err
	t.onFinish()
	fmt.Println(fmt.Sprintf("set error: %v", t.Index))
}

func (t *TestQueueTask) GetResult() interface{} {
	return t.Result
}

func (t *TestQueueTask) GetError() error {
	return t.Error
}

func (t *TestQueueTask) WithFinishFunc(fn func()) {
	t.onFinish = fn
}

func NewTestQueueTasks(n int) (tasks []QueueTask) {
	until := time.Now().Add(time.Second)
	tasks = make([]QueueTask, 0, n)

	for i := 0; i < n; i++ {
		task := &TestQueueTask{
			Index:     i,
			Partition: "partition",
			Payload:   fmt.Sprintf("payload %v", i),
			Until:     until,
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func TestMemoryBatchTaskQueue(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	bsp := new(TestBatchSizeProvider)
	var hdl = func(pid string, tasks []QueueTask) {
		time.Sleep(time.Duration(rand.Int63n(100)) * time.Millisecond)
		for _, v := range tasks {
			r := fmt.Sprintf("%v-%v", pid, time.Now().UnixNano())
			v.SetResult(r)
		}
	}
	q := NewMemoryBatchQueue(bsp, hdl, 10)

	{
		tasks := NewTestQueueTasks(0)
		finishC := q.Push(tasks...)
		<-finishC
		fmt.Println("batch finished")
	}

	{
		tasks := NewTestQueueTasks(5)
		finishC := q.Push(tasks...)
		<-finishC
		fmt.Println("batch finished")
	}

	{
		tasks1 := NewTestQueueTasks(20)
		finishC1 := q.Push(tasks1...)

		tasks2 := NewTestQueueTasks(3)
		finishC2 := q.Push(tasks2...)

		<-finishC1
		<-finishC2
		fmt.Println("batch finished")
	}
}
