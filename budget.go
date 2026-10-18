package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

type Status string

const (
	Pending    Status = "Pending"
	Processing Status = "Processing"
	Completed  Status = "Completed"
)

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidState = errors.New("task must be pending to process")
)

// All money amounts are whole KZT, so calculations do not use floating point.
type BudgetInput struct {
	EventName    string
	Guests       int64
	TicketPrice  int64
	FixedCosts   int64
	CostPerGuest int64
}

func (input BudgetInput) Validate() error {
	if strings.TrimSpace(input.EventName) == "" {
		return errors.New("event name must not be empty")
	}
	if input.Guests < 0 || input.TicketPrice < 0 || input.FixedCosts < 0 || input.CostPerGuest < 0 {
		return errors.New("guest count and money amounts must not be negative")
	}
	// Check before multiplying or adding to prevent integer overflow.
	if input.Guests > 0 {
		if input.TicketPrice > math.MaxInt64/input.Guests {
			return errors.New("revenue is too large")
		}
		if input.CostPerGuest > (math.MaxInt64-input.FixedCosts)/input.Guests {
			return errors.New("total costs are too large")
		}
	}
	return nil
}

type BudgetResult struct {
	Revenue           int64
	TotalCosts        int64
	Profit            int64
	BreakEvenTickets  int64
	BreakEvenPossible bool
}

type Task struct {
	ID     int
	Input  BudgetInput
	Status Status
	Result BudgetResult // Meaningful only when Status is Completed.
}

// Processor separates budget calculation from task storage and lifecycle.
type Processor interface {
	Calculate(BudgetInput) (BudgetResult, error)
}

type BudgetCalculator struct{}

func (BudgetCalculator) Calculate(input BudgetInput) (BudgetResult, error) {
	if err := input.Validate(); err != nil {
		return BudgetResult{}, err
	}
	result := BudgetResult{
		Revenue:    input.Guests * input.TicketPrice,
		TotalCosts: input.FixedCosts + input.Guests*input.CostPerGuest,
	}
	result.Profit = result.Revenue - result.TotalCosts
	margin := input.TicketPrice - input.CostPerGuest
	if input.FixedCosts == 0 {
		// With no fixed costs, selling zero tickets already avoids a loss.
		result.BreakEvenPossible = true
	} else if margin > 0 {
		result.BreakEvenPossible = true
		result.BreakEvenTickets = input.FixedCosts / margin
		if input.FixedCosts%margin != 0 {
			result.BreakEvenTickets++
		}
	}
	return result, nil
}

type TaskService struct {
	tasks     map[int]Task
	nextID    int
	processor Processor
}

func NewTaskService() *TaskService {
	return &TaskService{
		tasks:     make(map[int]Task),
		nextID:    1,
		processor: BudgetCalculator{},
	}
}

func (service *TaskService) Create(input BudgetInput) (Task, error) {
	if err := input.Validate(); err != nil {
		return Task{}, err
	}
	task := Task{ID: service.nextID, Input: input, Status: Pending}
	service.tasks[task.ID] = task
	service.nextID++
	return task, nil
}

// Return a value copy: callers cannot change a task stored in the map.
func (service *TaskService) Get(id int) (Task, error) {
	task, ok := service.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	return task, nil
}

func (service *TaskService) Process(id int) (Task, error) {
	task, err := service.Get(id)
	if err != nil {
		return Task{}, err
	}
	if task.Status != Pending {
		return Task{}, ErrInvalidState
	}
	task.Status = Processing
	service.tasks[id] = task
	result, err := service.processor.Calculate(task.Input)
	if err != nil {
		// A failed attempt returns to Pending and may be retried.
		task.Status = Pending
		service.tasks[id] = task
		return Task{}, err
	}
	task.Result = result
	task.Status = Completed
	service.tasks[id] = task
	return task, nil
}
