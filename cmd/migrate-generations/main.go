package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dbHost := flag.String("host", "localhost", "Database host")
	dbPort := flag.String("port", "5432", "Database port")
	dbUser := flag.String("user", "aifacebot_user", "Database user")
	dbPassword := flag.String("password", "aifacebot_password", "Database password")
	dbName := flag.String("db", "aifacebot", "Database name")
	dryRun := flag.Bool("dry-run", true, "Dry run mode (don't actually update)")

	flag.Parse()

	// Connect to database
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, *dbPort, *dbUser, *dbPassword, *dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Connected to database successfully")

	// Get all generation_requests with their current user_id
	rows, err := db.Query(`
		SELECT id, user_id FROM generation_requests
		ORDER BY id
	`)
	if err != nil {
		log.Fatalf("Failed to query generation_requests: %v", err)
	}
	defer rows.Close()

	type GenRequest struct {
		ID     int64
		UserID int64
	}

	var genRequests []GenRequest
	for rows.Next() {
		var gr GenRequest
		if err := rows.Scan(&gr.ID, &gr.UserID); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		genRequests = append(genRequests, gr)
	}

	log.Printf("Found %d generation requests\n", len(genRequests))

	// For each generation request, check if user_id is valid
	var needsUpdate []GenRequest
	var orphaned []GenRequest

	for _, gr := range genRequests {
		// Check if user_id exists in users table
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", gr.UserID).Scan(&exists)
		if err != nil {
			log.Fatalf("Failed to check user: %v", err)
		}

		if !exists {
			// Check if this user_id is actually a telegram_id
			var userID sql.NullInt64
			err := db.QueryRow("SELECT id FROM users WHERE telegram_id = $1", gr.UserID).Scan(&userID)
			if err == nil && userID.Valid {
				needsUpdate = append(needsUpdate, GenRequest{ID: gr.ID, UserID: userID.Int64})
				log.Printf("Gen %d: user_id %d -> %d (telegram_id match)\n", gr.ID, gr.UserID, userID.Int64)
			} else {
				orphaned = append(orphaned, gr)
				log.Printf("Gen %d: user_id %d is orphaned (no matching user)\n", gr.ID, gr.UserID)
			}
		}
	}

	log.Printf("\nSummary:")
	log.Printf("- Valid: %d\n", len(genRequests)-len(needsUpdate)-len(orphaned))
	log.Printf("- Need update: %d\n", len(needsUpdate))
	log.Printf("- Orphaned: %d\n", len(orphaned))

	if *dryRun {
		log.Println("\n[DRY RUN] No changes made. Run with -dry-run=false to apply changes.")
		os.Exit(0)
	}

	// Update generation_requests that need updating
	if len(needsUpdate) > 0 {
		log.Printf("\nUpdating %d generation requests...\n", len(needsUpdate))
		for _, gr := range needsUpdate {
			_, err := db.Exec("UPDATE generation_requests SET user_id = $1 WHERE id = $2", gr.UserID, gr.ID)
			if err != nil {
				log.Fatalf("Failed to update generation %d: %v", gr.ID, err)
			}
		}
		log.Printf("Updated %d generation requests\n", len(needsUpdate))
	}

	// Delete orphaned generation_requests
	if len(orphaned) > 0 {
		log.Printf("\nDeleting %d orphaned generation requests...\n", len(orphaned))
		for _, gr := range orphaned {
			_, err := db.Exec("DELETE FROM generation_requests WHERE id = $1", gr.ID)
			if err != nil {
				log.Fatalf("Failed to delete generation %d: %v", gr.ID, err)
			}
		}
		log.Printf("Deleted %d orphaned generation requests\n", len(orphaned))
	}

	log.Println("\nMigration completed successfully!")
}
