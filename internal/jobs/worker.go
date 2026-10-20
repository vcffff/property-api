package jobs

import (
	"fmt"
	"sync"
	"time"
)

type Work struct {
	ID int
}

func NewWorker(id int, jobs <-chan Work, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		fmt.Printf("Worker %d processing job %d\n", id, job.ID)
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("Worker %d finished job %d\n", id, job.ID)

	}
}
