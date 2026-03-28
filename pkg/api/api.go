package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jsjutzi/appt-scheduler/pkg/db"
	"github.com/jsjutzi/appt-scheduler/pkg/service"
)

type API struct {
	ApptService *service.ApptService
}

func NewAPI(apptService *service.ApptService) API {
	return API{ApptService: apptService}
}

func (a *API) SetupRoutes(r chi.Router) {
	r.Route("/appointments", func(r chi.Router) {
		// GET available appointment times
		r.Get("/available", a.getAvailableSlots)

		// POST new appointment
		r.Post("/appointments", a.bookAppointment)

		// GET scheduled appointments for a trainer
		r.Get("/appointments", a.getScheduledAppointments)

		// PUT update an existing appointment
		r.Put("/appointments/{id}", a.updateAppointment) // PUT /appointments/{id}

		// DELETE an appointment
		r.Delete("/appointments/{id}", a.deleteAppointment) // DELETE /appointments/{id}
	})
}

// getAvailableSlots handles GET /appointments/available?trainer_id=1&starts_at=2024-01-01T00:00:00Z&ends_at=2024-01-07T00:00:00Z
func (a *API) getAvailableSlots(w http.ResponseWriter, r *http.Request) {
	trainerIDStr := r.URL.Query().Get("trainer_id")
	startsAtStr := r.URL.Query().Get("starts_at")
	endsAtStr := r.URL.Query().Get("ends_at")

	trainerID, err := strconv.Atoi(trainerIDStr)
	if err != nil || trainerID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "valid trainer_id is required"})
		return
	}

	startsAt, err := time.Parse(time.RFC3339, startsAtStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid starts_at format. Use RFC3339"})
		return
	}

	endsAt, err := time.Parse(time.RFC3339, endsAtStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ends_at format. Use RFC3339"})
		return
	}

	slots, err := a.ApptService.GetAvailableSlots(r.Context(), trainerID, startsAt, endsAt)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, slots)
}

// bookAppointment handles POST /appointments
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
