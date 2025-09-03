# Smart Home Rule Tester

This script provides an easy way to test your smart home automation rules without running the full application stack.

## Usage

### Quick Test
Run a predefined test with sample data:
```bash
go run scripts/test_rules.go test
```

### Interactive Mode
Run the interactive tester:
```bash
go run scripts/test_rules.go
```

## Features

1. **Test Existing Rules**: Test rules stored in your database
2. **Custom Condition Testing**: Test individual conditions with custom values
3. **List All Rules**: View all available rules in your system
4. **Quick Test Mode**: Automated test with predefined data

## Requirements

- PostgreSQL database running on localhost:5432
- Redis running on localhost:6379
- Database connection string: `postgres://postgres:password@localhost:5432/smarthome?sslmode=disable`

## Example Output

```
🚀 Smart Home Rule Tester
=========================
Running quick test...
Testing rule rule1 with device temp_sensor1 (temperature: 35.0)
Rule: Temperature Alert
Enabled: true

Evaluating conditions...
Condition result: true
✅ Rule would trigger!
Actions that would execute:
  1. turn_on on device fan1
  2. send_notification on device mobile_app
```

## Database Setup

Make sure you have rules in your database. You can create them through the web interface or directly in the database.

Example rule structure:
- ID: rule1
- Name: Temperature Alert
- Conditions: JSON array of conditions
- Actions: JSON array of actions
- Enabled: true
