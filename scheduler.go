package main

import (
	"log"
	"runtime/debug"
	"time"
)

type ScheduleFunc func()

type SchedulerTask struct {
	Name     string
	Task     ScheduleFunc
	Interval time.Duration
	LastRun  time.Time
}

type Scheduler struct {
	Tasks      []*SchedulerTask
	TaskTicker *time.Ticker
	Resolution time.Duration
}

func NewScheduler(resolution time.Duration) *Scheduler {
	return &Scheduler{
		Tasks:      make([]*SchedulerTask, 0),
		Resolution: resolution,
	}
}

func (scheduler *Scheduler) Schedule(name string, task ScheduleFunc, interval time.Duration) {
	newTask := &SchedulerTask{
		Name:     name,
		Task:     task,
		Interval: interval,
		LastRun:  time.Time{},
	}

	scheduler.Tasks = append(scheduler.Tasks, newTask)
}

func (scheduler *Scheduler) Run() {
	doneChannel := make(chan bool)

	scheduler.TaskTicker = time.NewTicker(scheduler.Resolution)

	go scheduler.internalRun(doneChannel)

	// Block forever
	<-doneChannel
	close(doneChannel)
}

func (scheduler *Scheduler) internalRun(done chan bool) {
	for tick := range scheduler.TaskTicker.C {
		for _, task := range scheduler.Tasks {
			if task.shouldTaskRun(tick) {
				task.LastRun = tick
				task.run(tick)
			}
		}
	}

	done <- true
}

func (task *SchedulerTask) shouldTaskRun(current time.Time) bool {
	return current.Sub(task.LastRun) > task.Interval
}

func (task *SchedulerTask) run(current time.Time) {
	go task.wrapTaskRun()
}

func (task *SchedulerTask) taskRecover() {
	if r := recover(); r != nil {
		log.Printf("Panic occurred when running Task [%s]. Exception: %s", task.Name, r)
		debug.PrintStack()
	}
}

func (task *SchedulerTask) wrapTaskRun() {
	defer task.taskRecover()
	startTime := time.Now()

	task.Task()

	elapsedDuration := time.Now().Sub(startTime)
	log.Printf("Job [%s] ran succesfully in %s", task.Name, elapsedDuration.String())
}
