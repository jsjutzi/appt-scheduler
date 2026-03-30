package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jsjutzi/appt-scheduler/pkg/db"
	"github.com/jsjutzi/appt-scheduler/pkg/service"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Appointment Scheduler API
// @version 1.0
// @description Go-based API for managing trainer appointments with strict Pacific Time business rules.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
// @schemes http

type API struct {
	ApptService *service.ApptService
}

func NewAPI(apptService *service.ApptService) API {
	return API{ApptService: apptService}
}

func (a *API) SetupRoutes(r chi.Router) {
	// Swagger Documentation
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"), // point to the generated doc
	))

	// GET available appointment times
	r.Get("/available/", a.getAvailableSlots)

	r.Route("/appointments", func(r chi.Router) {
		// POST new appointment
		r.Post("/", a.bookAppointment)

		// GET scheduled appointments for a trainer
		r.Get("/", a.getScheduledAppointments)

		// PUT update an existing appointment
		r.Put("/{id}", a.updateAppointment) // PUT /appointments/{id}

		// DELETE an appointment
		r.Delete("/{id}", a.deleteAppointment) // DELETE /appointments/{id}
	})
}

// getAvailableSlots handles GET /appointments/available?trainer_id=1&started_at=2024-01-01T00:00:00Z&ended_at=2024-01-07T00:00:00Z
// getAvailableSlots godoc
// @Summary      Get available appointment slots
// @Description  Returns all available 30-minute slots for a trainer in the given date range
// @Tags         appointments
// @Accept       json
// @Produce      json
// @Param        trainer_id  query     int     true  "Trainer ID"
// @Param        started_at   query     string  true  "Start datetime (RFC3339)"
// @Param        ended_at     query     string  true  "End datetime (RFC3339)"
// @Success      200  {array}   db.Appointment
// @Failure      400  {object}  map[string]string
// @Router       /available [get]
func (a *API) getAvailableSlots(w http.ResponseWriter, r *http.Request) {
	trainerIDStr := r.URL.Query().Get("trainer_id")
	startsAtStr := r.URL.Query().Get("started_at")
	endsAtStr := r.URL.Query().Get("ended_at")

	trainerID, err := strconv.Atoi(trainerIDStr)
	if err != nil || trainerID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "valid trainer_id is required"})
		return
	}

	startedAt, err := time.Parse(time.RFC3339, startsAtStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid started_at format. Use RFC3339"})
		return
	}

	endedAt, err := time.Parse(time.RFC3339, endsAtStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ended_at format. Use RFC3339"})
		return
	}

	slots, err := a.ApptService.GetAvailableSlots(r.Context(), trainerID, startedAt, endedAt)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, slots)
}

// bookAppointment handles POST /appointments
// bookAppointment godoc
// @Summary      Book a new appointment
// @Description  Creates a new 30-minute appointment after validation
// @Tags         appointments
// @Accept       json
// @Produce      json
// @Param        appointment  body      db.Appointment  true  "Appointment object"
// @Success      201  {object}  db.Appointment
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string  "Slot already booked or validation failed"
// @Router       /appointments [post]
func (a *API) bookAppointment(w http.ResponseWriter, r *http.Request) {
	var appt db.Appointment
	if err := json.NewDecoder(r.Body).Decode(&appt); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON in request body"})
		return
	}

	created, err := a.ApptService.BookAppointment(r.Context(), appt)
	if err != nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

// getScheduledAppointments handles GET /appointments?trainer_id=1
// getScheduledAppointments godoc
// @Summary      Get scheduled appointments
// @Description  Returns all appointments for a specific trainer
// @Tags         appointments
// @Accept       json
// @Produce      json
// @Param        trainer_id  query  int  true  "Trainer ID"
// @Success      200  {array}  db.Appointment
// @Failure      400  {object} map[string]string
// @Router       /appointments [get]
func (a *API) getScheduledAppointments(w http.ResponseWriter, r *http.Request) {
	trainerIDStr := r.URL.Query().Get("trainer_id")
	trainerID, err := strconv.Atoi(trainerIDStr)
	if err != nil || trainerID <= 0 {
		http.Error(w, "invalid trainer_id", http.StatusBadRequest)
		return
	}

	appointments, err := a.ApptService.GetAppointmentsByTrainer(r.Context(), trainerID, time.Time{}, time.Time{})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, appointments)
}

// updateAppointment handles PUT /appointments/{id} - Update / Reschedule
// updateAppointment godoc
// @Summary      Update an appointment
// @Description  Reschedule an existing appointment
// @Tags         appointments
// @Accept       json
// @Produce      json
// @Param        id          path  int             true  "Appointment ID"
// @Param        appointment body  db.Appointment  true  "Updated appointment data"
// @Success      200  {object} db.Appointment
// @Failure      400  {object} map[string]string
// @Failure      404  {object} map[string]string
// @Failure      409  {object} map[string]string
// @Router       /appointments/{id} [put]
func (a *API) updateAppointment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "valid appointment id is required"})
		return
	}

	var appt db.Appointment
	if err := json.NewDecoder(r.Body).Decode(&appt); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON in request body"})
		return
	}

	// Make sure the ID in the URL matches the ID in the body
	appt.ID = id

	updatedAppt, err := a.ApptService.UpdateAppointment(r.Context(), appt)
	if err != nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, updatedAppt)
}

// deleteAppointment handles DELETE /appointments/{id} - Cancel appointment
// deleteAppointment godoc
// @Summary      Delete an appointment
// @Description  Cancel an existing appointment
// @Tags         appointments
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "Appointment ID"
// @Success      204
// @Failure      400  {object} map[string]string
// @Failure      404  {object} map[string]string
// @Router       /appointments/{id} [delete]
func (a *API) deleteAppointment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "valid appointment id is required"})
		return
	}

	if err := a.ApptService.DeleteAppointment(r.Context(), id); err != nil {
		if err.Error() == "appointment not found" {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "appointment not found"})
			return
		}

		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusNoContent, nil) // 204 No Content
}
