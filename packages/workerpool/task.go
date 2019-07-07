package workerpool

type Task struct {
	params     []interface{}
	resultChan chan interface{}
}

func (task *Task) Return(result interface{}) {
	task.resultChan <- result
	close(task.resultChan)
}

func (task *Task) Param(index int) interface{} {
	return task.params[index]
}
