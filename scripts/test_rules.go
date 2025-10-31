package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"smarthome/internal/db"
	"smarthome/internal/models"
	"smarthome/internal/redis"
	"smarthome/internal/taskqueue"
	"smarthome/internal/utils"

	goredis "github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("🚀 Smart Home Rule Tester")
	fmt.Println("=========================")

	// Check if we should run in test mode
	if len(os.Args) > 1 && os.Args[1] == "test" {
		runQuickTest()
		return
	}

	// Interactive mode
	runInteractiveTest()
}

func runQuickTest() {
	fmt.Println("Running quick test...")

	// Connect to database
	dbConn, err := db.NewDB("postgres://postgres:pass@localhost:5432/smarthome?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer dbConn.Close(context.Background())

	// Connect to Redis
	redisClient := redis.NewRedisClient("localhost:6379")

	// Set up task queue
	taskqueue.SetGlobalInstances(dbConn, redisClient, nil)

	// Test data
	testDeviceID := "temp_sensor1"
	testState := `{"temperature": 35.0}`

	// Update Redis
	redisClient.Set(context.Background(), fmt.Sprintf("device:%s", testDeviceID), testState, 0)

	// Test rule
	ruleID := "rule1"
	fmt.Printf("Testing rule %s with device %s (temperature: 35.0)\n", ruleID, testDeviceID)

	// Get rule from DB
	rule, err := dbConn.GetRuleByID(context.Background(), ruleID)
	if err != nil {
		log.Fatalf("Failed to get rule: %v", err)
	}

	fmt.Printf("Rule: %s\n", rule.Name)
	fmt.Printf("Enabled: %t\n", rule.Enabled)

	// Test condition evaluation
	fmt.Println("\nEvaluating conditions...")
	// Manually evaluate conditions for testing
	var condition models.Condition
	if err := json.Unmarshal(rule.Conditions, &condition); err != nil {
		log.Fatalf("Failed to unmarshal conditions: %v", err)
	}

	// Simple evaluation logic for testing
	result := evaluateTestCondition(condition, redisClient)
	fmt.Printf("Condition result: %t\n", result)

	if result {
		fmt.Println("✅ Rule would trigger!")
		fmt.Println("Actions that would execute:")
		var actions []models.Action
		json.Unmarshal(rule.Actions, &actions)
		for i, action := range actions {
			fmt.Printf("  %d. %s on device %s\n", i+1, action.Action, action.DeviceID)
		}
	} else {
		fmt.Println("❌ Rule would not trigger")
	}
}

func runInteractiveTest() {
	fmt.Println("Interactive Rule Tester")
	fmt.Println("1. Test existing rule")
	fmt.Println("2. Test custom condition")
	fmt.Println("3. Show all rules")
	fmt.Println("4. Exit")

	var choice int
	fmt.Print("Choose option: ")
	fmt.Scanln(&choice)

	switch choice {
	case 1:
		runQuickTest()
	case 2:
		testCustomCondition()
	case 3:
		showAllRules()
	case 4:
		return
	default:
		fmt.Println("Invalid choice")
	}
}

func testCustomCondition() {
	fmt.Println("Custom Condition Tester")

	var deviceID, key, op string
	var actualValue, expectedValue float64

	fmt.Print("Device ID: ")
	fmt.Scanln(&deviceID)
	fmt.Print("Key (e.g., temperature): ")
	fmt.Scanln(&key)
	fmt.Print("Actual value: ")
	fmt.Scanln(&actualValue)
	fmt.Print("Operator (> < == !=): ")
	fmt.Scanln(&op)
	fmt.Print("Expected value: ")
	fmt.Scanln(&expectedValue)

	result := utils.Compare(actualValue, op, expectedValue)
	fmt.Printf("Result: %t (%v %s %v)\n", result, actualValue, op, expectedValue)
}

func showAllRules() {
	fmt.Println("Available Rules:")

	dbConn, err := db.NewDB("postgres://postgres:password@localhost:5432/smarthome?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer dbConn.Close(context.Background())

	rules, err := dbConn.GetAllRules(context.Background())
	if err != nil {
		log.Fatalf("Failed to get rules: %v", err)
	}

	for _, rule := range rules {
		fmt.Printf("- %s (ID: %s, Enabled: %t)\n", rule.Name, rule.ID, rule.Enabled)
	}
}

// evaluateTestCondition is a simple test helper for condition evaluation
func evaluateTestCondition(cond models.Condition, redisClient *goredis.Client) bool {
	if cond.Operator == "" {
		switch cond.Type {
		case "sensor", "device":
			stateRaw, _ := redisClient.Get(context.Background(), fmt.Sprintf("device:%s", cond.DeviceID)).Result()
			var state utils.DeviceState
			json.Unmarshal([]byte(stateRaw), &state)

			var expectedValue interface{}
			if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
				return false
			}

			return utils.Compare(state[cond.Key], cond.Op, expectedValue)
		case "time":
			var expectedValue interface{}
			if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
				return false
			}
			return utils.Compare(utils.GetCurrentTime(), cond.Op, expectedValue)
		}
		return false
	}

	for _, child := range cond.Children {
		if cond.Operator == "AND" && !evaluateTestCondition(child, redisClient) {
			return false
		}
		if cond.Operator == "OR" && evaluateTestCondition(child, redisClient) {
			return true
		}
	}
	return cond.Operator == "AND"
}
