package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Appointment struct {
	ID        int       `json:"id" db:"id"`
	TrainerID int       `json:"trainerId" db:"trainer_id"`
	UserID    int       `json:"userId" db:"user_id"`
	StartsAt  time.Time `json:"startsAt" db:"starts_at"`
	EndsAt    time.Time `json:"endsAt" db:"ends_at"`
}

type ApptRepository interface {
	GetAppointmentsByTrainerID(ctx context.Context, trainerID int, startTime, endTime time.Time) ([]Appointment, error)
	GetAppointmentByID(ctx context.Context, apptID int) (Appointment, error)
	GetOverlappingAppointments(ctx context.Context, trainerID int, startsAt, endsAt time.Time) ([]Appointment, error)
	CreateAppointment(ctx context.Context, appt Appointment) (Appointment, error)
	UpdateAppointment(ctx context.Context, appt Appointment) (Appointment, error)
	DeleteAppointment(ctx context.Context, apptID int) error
	SeedFromJSON(ctx context.Context, filename string) error
}

type apptRepository struct {
	db *pgxpool.Pool
}

func NewApptRepository(db *pgxpool.Pool) ApptRepository {
	return &apptRepository{db: db}
}

// GetAppointmentsByTrainerID retrieves all appointments for a given trainer, optionally filtered by a date range.
func (ar *apptRepository) GetAppointmentsByTrainerID(ctx context.Context, trainerID int, startTime, endTime time.Time) ([]Appointment, error) {
	query := `
		SELECT id, trainer_id, user_id, starts_at, ends_at
		FROM appointments
		WHERE trainer_id = @trainerID
	`

	args := pgx.NamedArgs{
		"trainerID": trainerID,
	}

	hasDateFilter := !startTime.IsZero() && !endTime.IsZero()

	// Check if time/date filters are provided and add them to the query
	if hasDateFilter {
		// Catch overlaps not fully within the range
		query += ` 
			AND starts_at < @endTime 
			AND ends_at > @startTime
		`
		args["startTime"] = startTime
		args["endTime"] = endTime
	}

	// Order results (Can support pagination or sorting argument in the future if needed)
	query += " ORDER BY starts_at ASC"

	rows, err := ar.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("Error getting appts for trainer %w", err)
	}

	defer rows.Close()

	var appts []Appointment

	for rows.Next() {
		var appt Appointment
		if err := rows.Scan(
			&appt.ID,
			&appt.TrainerID,
			&appt.UserID,
			&appt.StartsAt,
			&appt.EndsAt,
		); err != nil {
			return nil, fmt.Errorf("Error scanning appt row %w", err)
		}

		appts = append(appts, appt)
	}

	// IMPORTANT: Check for errors that occurred *during* iteration.
	// rows.Next() returns false on both "no more rows" and "error happened".
	// rows.Err() surfaces any error that occurred while Postgres was streaming results.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating appointment rows for trainer %d: %w", trainerID, err)
	}

	return appts, nil
}

// CreateAppointment inserts a new appointment into the database and returns the created record with its assigned ID.
func (ar *apptRepository) CreateAppointment(ctx context.Context, appt Appointment) (Appointment, error) {
	query := `
		INSERT INTO appointments (trainer_id, user_id, starts_at, ends_at)
		VALUES (@trainerID, @userID, @startsAt, @endsAt)
		RETURNING id, trainer_id, user_id, starts_at, ends_at
	`
	args := pgx.NamedArgs{
		"trainerID": appt.TrainerID,
		"userID":    appt.UserID,
		"startsAt":  appt.StartsAt,
		"endsAt":    appt.EndsAt,
	}

	var createdAppt Appointment

	err := ar.db.QueryRow(ctx, query, args).Scan(
		&createdAppt.ID,
		&createdAppt.TrainerID,
		&createdAppt.UserID,
		&createdAppt.StartsAt,
		&createdAppt.EndsAt,
	)
	if err != nil {
		return Appointment{}, fmt.Errorf("Error creating appointment: %w", err)
	}

	return createdAppt, nil
}

// GetOverlappingAppointments checks for any appointments that overlap with the given time range for a specific trainer.
func (ar *apptRepository) GetOverlappingAppointments(ctx context.Context, trainerID int, startsAt, endsAt time.Time) ([]Appointment, error) {
	query := `
		SELECT id, trainer_id, user_id, starts_at, ends_at
		FROM appointments
		WHERE trainer_id = @trainerID
			AND starts_at < @endsAt
			AND ends_at > @startsAt
		ORDER BY starts_at ASC
	`

	args := pgx.NamedArgs{
		"trainerID": trainerID,
		"startsAt":  startsAt,
		"endsAt":    endsAt,
	}

	rows, err := ar.db.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("Error getting overlapping appts for trainer %d: %w", trainerID, err)
	}
	defer rows.Close()

	var appts []Appointment

	for rows.Next() {
		var appt Appointment
		if err := rows.Scan(
			&appt.ID,
			&appt.TrainerID,
			&appt.UserID,
			&appt.StartsAt,
			&appt.EndsAt,
		); err != nil {
			return nil, fmt.Errorf("Error scanning overlapping appt row %w", err)
		}

		appts = append(appts, appt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlapping appointment rows for trainer %d: %w", trainerID, err)
	}

	return appts, nil
}

// SeedFromJSON loads the initial appointments from appointments.json and inserts them
// into the database.
func (ar *apptRepository) SeedFromJSON(ctx context.Context, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		// If the file doesn't exist, we treat it as non-fatal (useful for production)
		if os.IsNotExist(err) {
			fmt.Printf("Warning: seed file %s not found, skipping seeding\n", filename)
			return nil
		}
		return fmt.Errorf("failed to read seed file %s: %w", filename, err)
	}

	var seedAppointments []Appointment
	if err := json.Unmarshal(data, &seedAppointments); err != nil {
		return fmt.Errorf("failed to parse appointments.json: %w", err)
	}

	pacificLoc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return fmt.Errorf("failed to load Pacific timezone: %w", err)
	}

	for _, appt := range seedAppointments {
		// Normalize time to Pacific Time zone per requirement.
		// This ensures the stored time is always represented in the correct location,
		// even if the JSON had a different offset.
		appt.StartsAt = appt.StartsAt.In(pacificLoc)
		appt.EndsAt = appt.EndsAt.In(pacificLoc)

		query := `
			INSERT INTO appointments (trainer_id, user_id, starts_at, ends_at)
			VALUES (@trainerID, @userID, @startsAt, @endsAt)
			ON CONFLICT (trainer_id, starts_at) DO NOTHING
		`

		args := pgx.NamedArgs{
			"trainerID": appt.TrainerID,
			"userID":    appt.UserID,
			"startsAt":  appt.StartsAt,
			"endsAt":    appt.EndsAt,
		}

		_, err := ar.db.Exec(ctx, query, args)
		if err != nil {
			return fmt.Errorf("failed to seed appointment for trainer %d at %s: %w",
				appt.TrainerID, appt.StartsAt.Format(time.RFC3339), err)
		}
	}

	fmt.Printf("Successfully seeded %d appointments from %s\n", len(seedAppointments), filename)
	return nil
}

// GetAppointmentByID retrieves a single appointment by its ID
func (ar *apptRepository) GetAppointmentByID(ctx context.Context, id int) (Appointment, error) {
	query := `
		SELECT id, trainer_id, user_id, starts_at, ends_at
		FROM appointments
		WHERE id = @id
	`

	args := pgx.NamedArgs{"id": id}

	var appt Appointment
	err := ar.db.QueryRow(ctx, query, args).Scan(
		&appt.ID,
		&appt.TrainerID,
		&appt.UserID,
		&appt.StartsAt,
		&appt.EndsAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Appointment{}, fmt.Errorf("appointment not found")
		}
		return Appointment{}, fmt.Errorf("failed to get appointment %d: %w", id, err)
	}
	return appt, nil
}

// UpdateAppointment updates an existing appointment (useful for rescheduling)
func (ar *apptRepository) UpdateAppointment(ctx context.Context, appt Appointment) (Appointment, error) {
	query := `
		UPDATE appointments
		SET trainer_id = @trainerID,
		    user_id = @userID,
		    starts_at = @startsAt,
		    ends_at = @endsAt
		WHERE id = @id
	`

	args := pgx.NamedArgs{
		"id":        appt.ID,
		"trainerID": appt.TrainerID,
		"userID":    appt.UserID,
		"startsAt":  appt.StartsAt,
		"endsAt":    appt.EndsAt,
	}

	var updated Appointment
	err := ar.db.QueryRow(ctx, query, args).Scan(
		&updated.ID, &updated.TrainerID, &updated.UserID, &updated.StartsAt, &updated.EndsAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return Appointment{}, fmt.Errorf("appointment not found")
		}

		return Appointment{}, fmt.Errorf("failed to update appointment %d: %w", appt.ID, err)
	}

	return updated, nil
}

// DeleteAppointment removes an appointment by ID
func (ar *apptRepository) DeleteAppointment(ctx context.Context, id int) error {
	query := `DELETE FROM appointments WHERE id = @id`

	args := pgx.NamedArgs{"id": id}

	result, err := ar.db.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("failed to delete appointment %d: %w", id, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("appointment not found")
	}

	return nil
}
