package g2cache

// thank https://github.com/ivpusic/grpool
import (
	"runtime"
	"sync"
	"time"
)

// Gorouting instance which can accept client jobs
type worker struct {
	id   int64
	pool *Pool
}

func (w *worker) start() {
	go func() {
		if CacheDebug {
			LogDebugF("Pool [%d] worker start\n", w.id)
		}
		defer func() {
			w.pool.wg.Done()
		}()
		for {
			select {
			case <-w.pool.stopped:
				if CacheDebug {
					LogDebugF("Pool [%d] worker <-stop\n", w.id)
				}
				if len(w.pool.jobQueue) != 0 {
					for job := range w.pool.jobQueue {
						runJob(w.id, job)
					}
				}
				if CacheDebug {
					LogDebugF("Pool [%d] worker exit\n", w.id)
				}
				return
			case job, ok := <-w.pool.jobQueue:
				if ok {
					runJob(w.id, job)
				}
			}
		}
	}()
}

func runJob(id int64, f func()) {
	defer func() {
		if err := recover(); err != nil {
			if CacheDebug {
				LogErrF("Pool [%d] Job panic err: %v, stack: %v\n", id, err,string(outputStackErr()))
			}
		}
	}()
	f()
}

func outputStackErr() []byte  {
	var (
		buf    [4096]byte
	)
	n := runtime.Stack(buf[:], false)
	return buf[:n]
}


func newWorker(id int64, pool *Pool) *worker {
	w := &worker{
		id:   id,
		pool: pool,
	}
	w.start()
	return w
}

// Represents user request, function which should be executed in some worker.
type Job func()

type Pool struct {
	jobQueue chan Job
	workers  []*worker
	stopOne  sync.Once
	stopped  chan struct{}
	wg       sync.WaitGroup
}

// Will make pool of gorouting workers.
// numWorkers - how many workers will be created for this pool
// queueLen - how many jobs can we accept until we block
//
// Returned object contains JobQueue reference, which you can use to send job to pool.
func NewPool(numWorkers int, jobQueueLen int) *Pool {

	pool := &Pool{
		jobQueue: make(chan Job, jobQueueLen),
		workers:  make([]*worker, numWorkers),
		stopped:  make(chan struct{}),
	}

	for i := 0; i < numWorkers; i++ {
		pool.wg.Add(1)
		pool.workers[i] = newWorker(int64(i), pool)
	}

	if CacheMonitor {
		pool.wg.Add(1)
		go pool.monitor()
	}

	return pool
}

func (p *Pool) wrapJob(job func()) func() {
	return job
}

func (p *Pool) SendJobWithTimeout(job func(), t time.Duration) bool {
	select {
	case <-p.stopped:
		return false
	case <-time.After(t):
		return false
	case p.jobQueue <- p.wrapJob(job):
		return true
	}
}

func (p *Pool) SendJobWithDeadline(job func(), t time.Time) bool {
	s := t.Sub(time.Now())
	if s <= 0 {
		s = time.Second // timeout
	}
	select {
	case <-p.stopped:
		return false
	case <-time.After(s):
		return false
	case p.jobQueue <- p.wrapJob(job):
		return true
	}
}

func (p *Pool) SendJob(job func()) {
	select {
	case p.jobQueue <- p.wrapJob(job):
	case <-p.stopped:
		return
	}
}

func (p *Pool) monitor() {
	t := time.NewTicker(time.Duration(CacheMonitorSecond) * time.Second)
	for {
		select {
		case <-p.stopped:
			t.Stop()
			return
		case <-t.C:
			LogDebug("Pool jobQueue current len ", len(p.jobQueue))
		}
	}
}

func (p *Pool) release() {
	close(p.stopped)
	force := make(chan struct{})
	forceOne := sync.Once{}
	go func() {
		for {
			select {
			case <-force:
				return
			default:
				p.wg.Wait() // why always some goroutine not exit,who found bug
				forceOne.Do(func() {
					close(force)
				})
				return
			}
		}
	}()
	// forceExit
	time.AfterFunc(5*time.Second, func() {
		forceOne.Do(func() {
			close(force)
		})
	})
	<-force
	close(p.jobQueue)
}

// Will release resources used by pool
func (p *Pool) Release() {
	p.stopOne.Do(p.release)
}
