package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
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
)

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
		Enabled:  pm.IsEnabled(clickupPluginID),
		Token:    strings.TrimSpace(token),
		TeamID:   strings.TrimSpace(team),
		Interval: interval,
	}
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

// handleClickUpMentions lists stored mentions.
//
//	detected_after  ms epoch: only items detected after it (what the dashboard has not shown yet)
//	since_hours     only comments written in the last N hours
//	include_answered=1 also items the user already answered
func (s *Server) handleClickUpMentions(w http.ResponseWriter, r *http.Request) {
	if s.clickupStore == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "la integración de ClickUp no está disponible; revisa el log del daemon"})
		return
	}
	q := r.URL.Query()
	f := clickup.MentionFilter{Limit: clickupListLimit, IncludeAnswered: q.Get("include_answered") == "1" || q.Get("include_answered") == "true"}
	if ms, err := strconv.ParseInt(q.Get("detected_after"), 10, 64); err == nil && ms > 0 {
		f.DetectedAfter = time.UnixMilli(ms)
	}
	if h, err := strconv.Atoi(q.Get("since_hours")); err == nil && h > 0 {
		if h > clickupMaxSinceHr {
			h = clickupMaxSinceHr
		}
		f.Since = time.Now().Add(-time.Duration(h) * time.Hour)
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"enabled":  s.pluginMgr.IsEnabled(clickupPluginID),
		"status":   s.clickupStore.Status(),
		"mentions": s.clickupStore.Mentions(f),
	})
}

// handleClickUpPoll runs a poll right away and waits for it.
func (s *Server) handleClickUpPoll(w http.ResponseWriter, r *http.Request) {
	if s.clickupPoller == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "la integración de ClickUp no está disponible; revisa el log del daemon"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), clickupPollWait)
	defer cancel()

	res, err := s.clickupPoller.Poll(ctx)
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
