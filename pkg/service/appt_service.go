package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jsjutzi/appt-scheduler/pkg/db"
	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/us"
)

// pacificLoc handles Pacific Time correctly (including DST)
var pacificLoc, _ = time.LoadLocation("America/Los_Angeles")

var usCalendar = cal.NewBusinessCalendar()

func init() {
	usCalendar.AddHoliday(us.Holidays...)
}

// ApptService provides business logic for managing appointments, including validation of business rules and interaction with the repository layer.
type ApptService struct {
	repo db.ApptRepository
}

// NewApptService creates a new instance of ApptService with the provided repository.
func NewApptService(repo db.ApptRepository) *ApptService {
	return &ApptService{repo: repo}
}

// validateAppt checks if the appointment meets the business rules:
// 1. Must be exactly 30 minutes long.
// 2. Must start on :00 or :30 minutes past the hour.
// 3. Can only be scheduled Monday through Friday.
// 4. Must be between 8:00 AM and 5:00 PM Pacific Time.
// 5. Must not be on a US public holiday (this was not in requirements, but is a common business rule for scheduling).
func (s *ApptService) validateAppt(appt db.Appointment) error {
	// Normalize to PST
	startPT := appt.StartedAt.In(pacificLoc)
	endPT := appt.EndedAt.In(pacificLoc)

	// Must be exactly 30 minutes
	if endPT.Sub(startPT) != 30*time.Minute {
		return fmt.Errorf("appointment must be exactly 30 minutes long")
	}

	// Must start on :00 or :30
	minute := startPT.Minute()
	if minute != 0 && minute != 30 {
		return fmt.Errorf("appointment must start at :00 or :30 minutes past the hour")
	}

	// Monday to Friday only
	weekday := startPT.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return fmt.Errorf("appointments can only be scheduled Monday through Friday")
	}

	// Business hours 8:00 AM - 5:00 PM Pacific Time
	hour := startPT.Hour()
	isValidTime := (hour >= 8 && hour < 16) || (hour == 16 && minute == 30)

	if !isValidTime {
		return fmt.Errorf("appointments must be between 8:00 AM and 5:00 PM Pacific Time")
	}

	// Check for US public holiday
	actual, observed, holiday := usCalendar.IsHoliday(startPT)
	if actual || observed {
		return fmt.Errorf("appointments cannot be scheduled on US public holidays: %s", holiday)
	}

	return nil
}

// GetAvailableSlots returns all valid 30-minute slots between two dates
// that are not already booked for the given trainer.
func (s *ApptService) GetAvailableSlots(ctx context.Context, trainerID int, rangeStart, rangeEnd time.Time) ([]db.Appointment, error) {
	// Normalize to Pacific Time
	rangeStart = rangeStart.In(pacificLoc)
	rangeEnd = rangeEnd.In(pacificLoc)

	// Get all currently booked appts that could overlap this range
	bookedAppts, err := s.repo.GetAppointmentsByTrainerID(ctx, trainerID, rangeStart, rangeEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get booked appointments: %w", err)
	}

	var availableSlots []db.Appointment

	currentDay := rangeStart.Truncate(24 * time.Hour)

	for currentDay.Before(rangeEnd) || currentDay.Equal(rangeEnd) {
		weekday := currentDay.Weekday()

		// Skip weekends
		if weekday != time.Saturday && weekday != time.Sunday {
			// Check each 30-minute slot during business hours
			for hour := 8; hour <= 16; hour++ {
				for minute := 0; minute < 60; minute += 30 {
					slotStart := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), hour, minute, 0, 0, pacificLoc)
					slotEnd := slotStart.Add(30 * time.Minute)

					// Skip if slot ends after 5:00pm
					if slotEnd.Hour() > 17 || (slotEnd.Hour() == 17 && slotEnd.Minute() > 0) {
						continue
					}

					// Skip if slot is before the requested range
					if slotStart.Before(rangeStart) {
						continue
					}

					if slotStart.After(rangeEnd) {
						break
					}

					// Skip if it's a US Holiday
					actual, observed, _ := usCalendar.IsHoliday(slotStart)
					if actual || observed {
						continue
					}

					isBooked := false
					for _, appt := range bookedAppts {
						if slotStart.Before(appt.EndedAt) && slotEnd.After(appt.StartedAt) {
							isBooked = true
							break
						}
					}

					if !isBooked {
						availableSlots = append(availableSlots, db.Appointment{
							TrainerID: trainerID,
							StartedAt: slotStart,
							EndedAt:   slotEnd,
						})
					}

				}
			}
		}

		// Move to the next day
		currentDay = currentDay.Add(24 * time.Hour)
	}

	return availableSlots, nil
}

// BookAppointment validates business rules and creates a new appointment if valid
func (s *ApptService) BookAppointment(ctx context.Context, appt db.Appointment) (db.Appointment, error) {
	// Validate all business rules
	if err := s.validateAppt(appt); err != nil {
		return db.Appointment{}, fmt.Errorf("validation failed: %w", err)
	}

	// Check for overlapping appointments
	overlapping, err := s.repo.GetOverlappingAppointments(ctx, appt.TrainerID, appt.StartedAt, appt.EndedAt)
	if err != nil {
		return db.Appointment{}, fmt.Errorf("failed to check for overlaps: %w", err)
	}
	if len(overlapping) > 0 {
		return db.Appointment{}, fmt.Errorf("time slot is already booked")
	}

	// Create the appointment
	created, err := s.repo.CreateAppointment(ctx, appt)
	if err != nil {
		return db.Appointment{}, fmt.Errorf("failed to create appointment: %w", err)
	}

	return created, nil
}

// GetAppointmentsByTrainer returns all appointments for a trainer (optionally in a date range)
// Getting all appointments without a date range has the potential to return a lot of data, so in a real system you'd likely want to enforce some limits or pagination here.
// For simplicity, we'll allow it but it's something to be aware of.
func (s *ApptService) GetAppointmentsByTrainer(ctx context.Context, trainerID int, startTime, endTime time.Time) ([]db.Appointment, error) {
	return s.repo.GetAppointmentsByTrainerID(ctx, trainerID, startTime, endTime)
}

// GetAppointmentByID retrieves a single appointment (used by update/delete)
func (s *ApptService) GetAppointmentByID(ctx context.Context, id int) (db.Appointment, error) {
	return s.repo.GetAppointmentByID(ctx, id)
}

// UpdateAppointment allows rescheduling an existing appointment
// It performs full validation and overlap checking on the new time
func (s *ApptService) UpdateAppointment(ctx context.Context, appt db.Appointment) (db.Appointment, error) {
	if appt.ID == 0 {
		return db.Appointment{}, fmt.Errorf("appointment id is required")
	}

	// Validate the new time against all business rules
	if err := s.validateAppt(appt); err != nil {
		return db.Appointment{}, fmt.Errorf("validation failed: %w", err)
	}

	// Get the existing appointment to compare
	existing, err := s.repo.GetAppointmentByID(ctx, appt.ID)
	if err != nil {
		return db.Appointment{}, fmt.Errorf("failed to fetch existing appointment: %w", err)
	}

	// If the time is actually changing, check for overlaps with other appointments
	// (ignore overlap with itself)
	if !existing.StartedAt.Equal(appt.StartedAt) || !existing.EndedAt.Equal(appt.EndedAt) {
		overlapping, err := s.repo.GetOverlappingAppointments(ctx, appt.TrainerID, appt.StartedAt, appt.EndedAt)
		if err != nil {
			return db.Appointment{}, fmt.Errorf("failed to check overlaps: %w", err)
		}

		// Generate filtered list of overlapping appointments that excludes the current appointment being updated
		filtered := make([]db.Appointment, 0, len(overlapping))
		for _, a := range overlapping {
			if a.ID != appt.ID {
				filtered = append(filtered, a)
			}
		}

		overlapping = filtered
	}

	updatedAppt, err := s.repo.UpdateAppointment(ctx, appt)
	if err != nil {
		return db.Appointment{}, fmt.Errorf("failed to update appointment: %w", err)
	}

	// Perform the update
	return updatedAppt, nil
}

// DeleteAppointment cancels an existing appointment
func (s *ApptService) DeleteAppointment(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("valid appointment id is required")
	}

	appt, err := s.repo.GetAppointmentByID(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().In(pacificLoc)

	// Cannot cancel past appointments
	if appt.StartedAt.Before(now) {
		return fmt.Errorf("cannot cancel past appointments")
	}

	minNotice := 30 * time.Minute // I chose 30 minutes but this can be any length of time
	if appt.StartedAt.Sub(now) < minNotice {
		return fmt.Errorf("appointments must be cancelled at least %v in advance", minNotice)
	}

	return s.repo.DeleteAppointment(ctx, id)
}
