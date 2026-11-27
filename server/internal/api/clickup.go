package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"orbit/internal/calendar"
	"orbit/internal/clickup"
	"orbit/internal/plugin"
)

const (
	clickupPluginID   = "clickup"
	clickupStoreFile  = "clickup_mentions.json"
	clickupPollWait   = 150 * time.Second
	clickupListLimit  = 100
	clickupMaxSinceHr = 24 * 30
	// clickupMaxLookbackHr bounds a manual "check the last N hours" poll (clickup.MaxLookback).
	clickupMaxLookbackHr = int(clickup.MaxLookback / time.Hour)
)

// clickupCommentID matches the comment IDs the inbox actions accept.
var clickupCommentID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// newClickUpPoller wires the ClickUp poller to the plugin config, the daily notes and the event hub.
// It returns nil (feature unavailable) when the store cannot be opened.
func newClickUpPoller(dataDir string, pluginMgr *plugin.Manager, notes clickup.NoteSink, events *eventHub) (*clickup.Store, *clickup.Poller) {
	store, err := clickup.OpenStore(filepath.Join(dataDir, clickupStoreFile))
	if err != nil {
		log.Printf("[clickup] integración desactivada: %v", err)
		return nil, nil
	}
	poller := clickup.NewPoller(clickup.Options{
		Store:    store,
		Notes:    notes,
		Config:   func() clickup.Config { return clickupConfigFrom(pluginMgr) },
		Notify:   func() { events.publish(eventKindClickUp) },
		NoteDate: calendar.GetEffectiveHandoffDate,
	})
	return store, poller
}

// clickupConfigFrom reads the plugin settings saved from Orbit > Plugins.
func clickupConfigFrom(pm *plugin.Manager) clickup.Config {
	raw := pm.GetConfig(clickupPluginID)
	token, _ := raw["api_key"].(string)
	team, _ := raw["workspace_id"].(string)

	interval := clickup.DefaultInterval
	if minutes := numberOf(raw["interval_minutes"]); minutes > 0 {
		interval = time.Duration(minutes * float64(time.Minute))
	}
	if interval < clickup.MinInterval {
		interval = clickup.MinInterval
	}
	return clickup.Config{
		Enabled:   pm.IsEnabled(clickupPluginID),
		Token:     strings.TrimSpace(token),
		TeamID:    strings.TrimSpace(team),
		Interval:  interval,
		DailyNote: boolOf(raw["daily_note"]),
	}
}

func boolOf(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		s := strings.ToLower(strings.TrimSpace(b))
		return s == "true" || s == "1"
	}
	return false
}

func numberOf(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f
	}
	return 0
}

// StartBackgroundJobs launches the daemon's periodic jobs. Tests that only need the routes skip it.
func (s *Server) StartBackgroundJobs() {
	if s.clickupPoller != nil {
		s.clickupPoller.Start()
	}
}

// StopBackgroundJobs stops them, cancelling work in flight.
func (s *Server) StopBackgroundJobs() {
	if s.clickupPoller != nil {
		s.clickupPoller.Stop()
	}
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// handleClickUpMentions lists stored mentions: by default the pending ones (the inbox).
//
//	detected_after  ms epoch + after_id (comment ID): cursor of what the dashboard already showed;
//	                returns items strictly after it, ordered by detection time then comment ID
//	since_hours     only comments written in the last N hours
//	include_answered=1 also items already answered or marked attended
func (s *Server) handleClickUpMentions(w http.ResponseWriter, r *http.Request) {
	if s.clickupStore == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "la integración de ClickUp no está disponible; revisa el log del daemon"})
		return
	}
	q := r.URL.Query()
	includeResolved := boolOf(q.Get("include_answered"))
	f := clickup.MentionFilter{Limit: clickupListLimit, IncludeResolved: includeResolved}
	if ms, err := strconv.ParseInt(q.Get("detected_after"), 10, 64); err == nil && ms > 0 {
		f.DetectedAfter = time.UnixMilli(ms)
		f.AfterID = q.Get("after_id")
	}
	if h, err := strconv.Atoi(q.Get("since_hours")); err == nil && h > 0 {
		if h > clickupMaxSinceHr {
			h = clickupMaxSinceHr
		}
		f.Since = time.Now().Add(-time.Duration(h) * time.Hour)
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"enabled":      s.pluginMgr.IsEnabled(clickupPluginID),
		"dailyNote":    clickupConfigFrom(s.pluginMgr).DailyNote,
		"status":       s.clickupStore.Status(),
		"pendingCount": s.clickupStore.PendingCount(),
		"mentions":     s.clickupStore.Mentions(f),
	})
}

// handleClickUpMentionDone marks a mention attended ({"done": true}, also an empty body) or pending
// again ({"done": false}).
func (s *Server) handleClickUpMentionDone(w http.ResponseWriter, r *http.Request) {
	if s.clickupStore == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "la integración de ClickUp no está disponible; revisa el log del daemon"})
		return
	}
	id := r.PathValue("id")
	if !clickupCommentID.MatchString(id) {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "ID de comentario inválido"})
		return
	}
	var body struct {
		Done *bool `json:"done"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "cuerpo inválido: se espera {\"done\": true|false}"})
		return
	}
	done := body.Done == nil || *body.Done

	m, err := s.clickupStore.SetDone(id, done, time.Now())
	switch {
	case errors.Is(err, clickup.ErrMentionNotFound):
		writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	case err != nil:
		writeJSONStatus(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	s.events.publish(eventKindClickUp) // other open pages update their inbox and badge
	writeJSONStatus(w, http.StatusOK, map[string]any{"mention": m, "pendingCount": s.clickupStore.PendingCount()})
}

// clickupPollOptions reads the optional body of POST /api/clickup/poll: {"lookback_hours": N}, an
// integer from 1 to clickupMaxLookbackHr. An empty body means a regular poll.
func clickupPollOptions(r *http.Request) (clickup.PollOptions, error) {
	var body struct {
		LookbackHours *float64 `json:"lookback_hours"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		return clickup.PollOptions{}, errors.New("cuerpo inválido: se espera {\"lookback_hours\": N}")
	}
	if body.LookbackHours == nil {
		return clickup.PollOptions{}, nil
	}
	h := *body.LookbackHours
	if h != float64(int(h)) || h < 1 || int(h) > clickupMaxLookbackHr {
		return clickup.PollOptions{}, errors.New("'lookback_hours' debe ser un entero entre 1 y " + strconv.Itoa(clickupMaxLookbackHr))
	}
	return clickup.PollOptions{Lookback: time.Duration(h) * time.Hour}, nil
}

// handleClickUpPoll runs a poll right away and waits for it.
func (s *Server) handleClickUpPoll(w http.ResponseWriter, r *http.Request) {
	if s.clickupPoller == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "la integración de ClickUp no está disponible; revisa el log del daemon"})
		return
	}
	opts, err := clickupPollOptions(r)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), clickupPollWait)
	defer cancel()

	res, err := s.clickupPoller.PollWith(ctx, opts)
	body := map[string]any{"result": res, "status": s.clickupStore.Status()}
	switch {
	case errors.Is(err, clickup.ErrDisabled):
		body["error"] = err.Error()
		writeJSONStatus(w, http.StatusConflict, body)
	case err != nil:
		body["error"] = err.Error()
		writeJSONStatus(w, http.StatusBadGateway, body)
	default:
		writeJSONStatus(w, http.StatusOK, body)
	}
}
