package main

import "fmt"

func main() {
	service := NewTaskService()
	inputs := []BudgetInput{
		{EventName: "Student Party", Guests: 100, TicketPrice: 5000, FixedCosts: 200000, CostPerGuest: 1000},
		{EventName: "Small Concert", Guests: 50, TicketPrice: 3000, FixedCosts: 200000, CostPerGuest: 1000},
	}
	for _, input := range inputs {
		task, err := service.Create(input)
		if err != nil {
			fmt.Println("Create error:", err)
			continue
		}
		fmt.Printf("Task %d: %s | %s\n", task.ID, task.Input.EventName, task.Status)
		if _, err := service.Process(task.ID); err != nil {
			fmt.Println("Process error:", err)
			continue
		}
		stored, err := service.Get(task.ID)
		if err != nil {
			fmt.Println("Get error:", err)
			continue
		}
		fmt.Println("Status:", stored.Status)
		fmt.Printf("Revenue: %d KZT\nTotal costs: %d KZT\nProfit: %d KZT\n",
			stored.Result.Revenue, stored.Result.TotalCosts, stored.Result.Profit)
		if stored.Result.BreakEvenPossible {
			fmt.Printf("Break-even tickets: %d\n\n", stored.Result.BreakEvenTickets)
		} else {
			fmt.Println("Break-even is impossible with these prices and costs.")
		}
	}
}
