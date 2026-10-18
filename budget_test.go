package main

import (
	"errors"
	"math"
	"testing"
)

func sampleInput() BudgetInput {
	return BudgetInput{EventName: "Party", Guests: 100, TicketPrice: 5000, FixedCosts: 200000, CostPerGuest: 1000}
}

func TestCreateAndGet(t *testing.T) {
	service := NewTaskService()
	input := sampleInput()
	task, err := service.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID <= 0 || task.Status != Pending || task.Input != input || task.Result != (BudgetResult{}) {
		t.Fatalf("unexpected new task: %+v", task)
	}
	stored, err := service.Get(task.ID)
	if err != nil || stored != task {
		t.Fatalf("Get = %+v, %v; want %+v", stored, err, task)
	}
	// Mutating a returned copy must not change the stored task.
	task.Input.EventName = "Changed"
	task.Status = Completed
	again, err := service.Get(stored.ID)
	if err != nil || again != stored {
		t.Fatalf("stored task changed through a returned copy: %+v, %v", again, err)
	}
}

func TestInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		input BudgetInput
	}{
		{"empty name", BudgetInput{}},
		{"blank name", BudgetInput{EventName: " \t\n"}},
		{"negative guests", BudgetInput{EventName: "Party", Guests: -1}},
		{"negative ticket price", BudgetInput{EventName: "Party", TicketPrice: -1}},
		{"negative fixed costs", BudgetInput{EventName: "Party", FixedCosts: -1}},
		{"negative guest cost", BudgetInput{EventName: "Party", CostPerGuest: -1}},
		{"revenue overflow", BudgetInput{EventName: "Party", Guests: 2, TicketPrice: math.MaxInt64}},
		{"cost overflow", BudgetInput{EventName: "Party", Guests: 1, FixedCosts: math.MaxInt64, CostPerGuest: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewTaskService()
			if _, err := service.Create(tc.input); err == nil {
				t.Fatal("expected validation error")
			}
			if len(service.tasks) != 0 {
				t.Fatal("invalid task was stored")
			}
			if _, err := (BudgetCalculator{}).Calculate(tc.input); err == nil {
				t.Fatal("calculator accepted invalid input")
			}
		})
	}
}

func TestMissingTask(t *testing.T) {
	service := NewTaskService()
	if _, err := service.Get(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v; want ErrNotFound", err)
	}
	if _, err := service.Process(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Process error = %v; want ErrNotFound", err)
	}
}

func TestBudgetResults(t *testing.T) {
	cases := []struct {
		name  string
		input BudgetInput
		want  BudgetResult
	}{
		{"profit", sampleInput(), BudgetResult{Revenue: 500000, TotalCosts: 300000, Profit: 200000, BreakEvenTickets: 50, BreakEvenPossible: true}},
		{"loss", BudgetInput{"Concert", 50, 3000, 200000, 1000}, BudgetResult{Revenue: 150000, TotalCosts: 250000, Profit: -100000, BreakEvenTickets: 100, BreakEvenPossible: true}},
		{"round up", BudgetInput{"Event", 4, 6, 10, 3}, BudgetResult{Revenue: 24, TotalCosts: 22, Profit: 2, BreakEvenTickets: 4, BreakEvenPossible: true}},
		{"zero margin", BudgetInput{"Event", 2, 1000, 5000, 1000}, BudgetResult{Revenue: 2000, TotalCosts: 7000, Profit: -5000}},
		{"negative margin", BudgetInput{"Event", 2, 500, 5000, 1000}, BudgetResult{Revenue: 1000, TotalCosts: 7000, Profit: -6000}},
		{"no fixed costs", BudgetInput{"Event", 2, 1000, 0, 500}, BudgetResult{Revenue: 2000, TotalCosts: 1000, Profit: 1000, BreakEvenPossible: true}},
		{"no guests", BudgetInput{"Event", 0, 5000, 200000, 1000}, BudgetResult{TotalCosts: 200000, Profit: -200000, BreakEvenTickets: 50, BreakEvenPossible: true}},
		{"zero budget", BudgetInput{EventName: "Free event"}, BudgetResult{BreakEvenPossible: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewTaskService()
			task, err := service.Create(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			done, err := service.Process(task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if done.Status != Completed || done.Result != tc.want {
				t.Fatalf("task = %+v; want Completed with %+v", done, tc.want)
			}
			stored, err := service.Get(task.ID)
			if err != nil || stored != done {
				t.Fatalf("result not stored: %+v, %v", stored, err)
			}
			if _, err := service.Process(task.ID); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("repeat processing error = %v; want ErrInvalidState", err)
			}
			after, err := service.Get(task.ID)
			if err != nil || after != done {
				t.Fatal("rejected operation changed completed task")
			}
		})
	}
}

func TestIndependentTasks(t *testing.T) {
	service := NewTaskService()
	first, err := service.Create(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := BudgetInput{"Concert", 50, 3000, 200000, 1000}
	second, err := service.Create(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("IDs must be unique")
	}
	if _, err := service.Process(first.ID); err != nil {
		t.Fatal(err)
	}
	untouched, err := service.Get(second.ID)
	if err != nil || untouched != second {
		t.Fatal("processing first task changed second task")
	}
	if _, err := service.Process(second.ID); err != nil {
		t.Fatal(err)
	}
	a, err := service.Get(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.Input != first.Input || b.Input != secondInput || a.Result.Profit != 200000 || b.Result.Profit != -100000 || a.Status != Completed || b.Status != Completed {
		t.Fatalf("tasks were mixed: %+v, %+v", a, b)
	}
}

// A small test processor lets the test observe the synchronous Processing state.
type observingProcessor struct {
	service *TaskService
	id      int
	t       *testing.T
	fail    bool
}

func (p observingProcessor) Calculate(input BudgetInput) (BudgetResult, error) {
	task, err := p.service.Get(p.id)
	if err != nil || task.Status != Processing {
		p.t.Fatalf("expected Processing during calculation; got %+v, %v", task, err)
	}
	if _, err := p.service.Process(p.id); !errors.Is(err, ErrInvalidState) {
		p.t.Fatalf("processing an in-progress task should fail; got %v", err)
	}
	if p.fail {
		return BudgetResult{}, errors.New("calculation failed")
	}
	return (BudgetCalculator{}).Calculate(input)
}

func TestProcessingLifecycle(t *testing.T) {
	for _, fail := range []bool{false, true} {
		service := NewTaskService()
		task, err := service.Create(sampleInput())
		if err != nil {
			t.Fatal(err)
		}
		service.processor = observingProcessor{service, task.ID, t, fail}
		_, processErr := service.Process(task.ID)
		stored, err := service.Get(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if fail {
			if processErr == nil || stored.Status != Pending || stored.Result != (BudgetResult{}) {
				t.Fatalf("failed attempt should return to Pending: %+v, %v", stored, processErr)
			}
			service.processor = BudgetCalculator{}
			if _, err := service.Process(task.ID); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
		} else if processErr != nil || stored.Status != Completed {
			t.Fatalf("successful attempt should complete: %+v, %v", stored, processErr)
		}
	}
}
