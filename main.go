package main

import (
	"os"

	qdb "github.com/rqure/qdb/src"
)

func getDatabaseAddress() string {
	addr := os.Getenv("QDB_ADDR")
	if addr == "" {
		addr = "redis:6379"
	}

	return addr
}

func main() {
	db := qdb.NewRedisDatabase(qdb.RedisDatabaseConfig{
		Address: getDatabaseAddress(),
	})

	dbWorker := qdb.NewDatabaseWorker(db)
	leaderElectionWorker := qdb.NewLeaderElectionWorker(db)
	schemaValidator := qdb.NewSchemaValidator(db)
	garageController := NewGarageController(db)
	ttsController := NewTTSController(db)

	schemaValidator.AddEntity("Root", "SchemaUpdateTrigger")
	schemaValidator.AddEntity("GarageController", "OpenTTS", "CloseTTS", "OpenReminderTTS", "OpenReminderInterval")
	schemaValidator.AddEntity("GarageDoor", "GarageDoorStatus", "ControlDevice", "StatusDevice", "OpenTrigger", "CloseTrigger")

	dbWorker.Signals.SchemaUpdated.Connect(qdb.Slot(schemaValidator.OnSchemaUpdated))
	leaderElectionWorker.AddAvailabilityCriteria(func() bool {
		return schemaValidator.IsValid()
	})

	dbWorker.Signals.Connected.Connect(qdb.Slot(leaderElectionWorker.OnDatabaseConnected))
	dbWorker.Signals.Disconnected.Connect(qdb.Slot(leaderElectionWorker.OnDatabaseDisconnected))
	dbWorker.Signals.SchemaUpdated.Connect(qdb.Slot(garageController.OnSchemaUpdated))
	dbWorker.Signals.SchemaUpdated.Connect(qdb.Slot(ttsController.OnSchemaUpdated))

	leaderElectionWorker.Signals.BecameLeader.Connect(qdb.Slot(garageController.OnBecameLeader))
	leaderElectionWorker.Signals.BecameLeader.Connect(qdb.Slot(ttsController.OnBecameLeader))
	leaderElectionWorker.Signals.LosingLeadership.Connect(qdb.Slot(garageController.OnLostLeadership))
	leaderElectionWorker.Signals.LosingLeadership.Connect(qdb.Slot(ttsController.OnLostLeadership))

	// Create a new application configuration
	config := qdb.ApplicationConfig{
		Name: "garage",
		Workers: []qdb.IWorker{
			dbWorker,
			leaderElectionWorker,
			garageController,
			ttsController,
		},
	}

	// Create a new application
	app := qdb.NewApplication(config)

	// Execute the application
	app.Execute()
}
