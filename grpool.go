package g2cache

// thank https://github.com/ivpusic/grpool
import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Gorouting instance which can accept client jobs
type worker struct {
	id         int64
	jobChannel chan Job
	pool       *Pool
}

func (w *worker) start() {
	go func() {
		if CacheDebug {
			log.Printf("[%d] gpool.worker start\n", w.id)
		}
		defer func() {
			w.pool.wg.Done()
		}()
		for {
			select {
			case <-w.pool.stopped:
				if CacheDebug {
					log.Printf("[%d] gpool.worker <-stop\n", w.id)
				}
				if len(w.jobChannel) != 0 {
					for job := range w.jobChannel {
						runJob(w.id, job)
					}
				}
				if CacheDebug {
					log.Printf("[%d] gpool.worker exit\n", w.id)
				}
				return
			case job, ok := <-w.jobChannel:
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
				log.Printf("[%d] Job panic err: %v\n", id, err)
			}
		}
	}()
	f()
}

func newWorker(id int64, pool *Pool, jobChannel chan Job) *worker {
	w := &worker{
		id:         id,
		jobChannel: jobChannel,
		pool:       pool,
	}
	w.start()
	return w
}

// Represents user request, function which should be executed in some worker.
type Job func()

type Pool struct {
	jobTotal int64
	JobQueue chan Job
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
	jobQueue := make(chan Job, jobQueueLen)

	pool := &Pool{
		JobQueue: jobQueue,
		workers:  make([]*worker, numWorkers),
		stopped:  make(chan struct{}),
	}

	for i := 0; i < numWorkers; i++ {
		pool.wg.Add(1)
		pool.workers[i] = newWorker(int64(i), pool, jobQueue)
	}

	if CacheMonitor {
		pool.wg.Add(1)
		go pool.monitor()
	}

	return pool
}

func (p *Pool) wrapJob(job func()) func() {
	return func() {
		defer func() {
			atomic.AddInt64(&p.jobTotal, -1)
		}()
		job()
	}
}

func (p *Pool) SendJobWithTimeout(job func(), t time.Duration) bool {
	select {
	case <-p.stopped:
		return false
	case <-time.After(t):
		return false
	case p.JobQueue <- p.wrapJob(job):
		atomic.AddInt64(&p.jobTotal, 1)
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
	case p.JobQueue <- p.wrapJob(job):
		atomic.AddInt64(&p.jobTotal, 1)
		return true
	}
}

func (p *Pool) SendJob(job func()) {
	select {
	case p.JobQueue <- p.wrapJob(job):
		atomic.AddInt64(&p.jobTotal, 1)
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
			log.Println("g pool jobTotal len ", atomic.LoadInt64(&p.jobTotal))
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
	close(p.JobQueue)
}

// Will release resources used by pool
func (p *Pool) Release() {
	p.stopOne.Do(p.release)
}
