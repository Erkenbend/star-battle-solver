package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// partyRoutes adds the party mode to mux:
//
//	GET  /party                    create or join a party
//	POST /party                    create one (form: n=6&n=7...), redirects to its lobby
//	GET  /party/{code}             lobby: players, results, ready
//	GET  /party/{code}/round/{num} the puzzle page of a round
//	POST /party/{code}/{action}    join, ready, progress, solve, giveup, leave (JSON)
//	GET  /party/{code}/events      the party's state as server-sent events
func partyRoutes(mux *http.ServeMux, pm *Parties) {
	mux.HandleFunc("GET /party", func(w http.ResponseWriter, r *http.Request) {
		writeParty(w, http.StatusOK, PartyPage{})
	})
	mux.HandleFunc("POST /party", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var sizes []int
		for _, s := range r.Form["n"] {
			if n, err := strconv.Atoi(s); err == nil {
				sizes = append(sizes, n)
			}
		}
		code, err := pm.Create(sizes)
		if err != nil {
			writeParty(w, errorStatus(err), PartyPage{Error: err.Error()})
			return
		}
		http.Redirect(w, r, "/party/"+code, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /party/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if NormalizeCode(code) != code {
			http.Redirect(w, r, "/party/"+NormalizeCode(code), http.StatusFound)
			return
		}
		if _, err := pm.Snapshot(code, ""); err != nil {
			writeParty(w, errorStatus(err), PartyPage{Error: fmt.Sprintf("Party %s gibt es nicht (mehr).", code)})
			return
		}
		writeParty(w, http.StatusOK, PartyPage{Code: code})
	})
	mux.HandleFunc("GET /party/{code}/round/{num}", func(w http.ResponseWriter, r *http.Request) {
		num, err := strconv.Atoi(r.PathValue("num"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		d, err := pm.RoundData(r.PathValue("code"), num)
		if err != nil {
			http.Error(w, err.Error(), errorStatus(err))
			return
		}
		writePage(w, d)
	})
	mux.HandleFunc("POST /party/{code}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
			Name  string `json:"name"`
			Ready bool   `json:"ready"`
			Round int    `json:"round"`
			Stars []int  `json:"stars"` // solve: star column per row
			Count int    `json:"count"` // progress: stars set
			Hints int    `json:"hints"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
			return
		}
		code := r.PathValue("code")
		out := map[string]any{}
		var err error
		switch r.PathValue("action") {
		case "join":
			req.Token, out["id"], err = pm.Join(code, req.Name, req.Token)
			out["token"] = req.Token
		case "ready":
			err = pm.SetReady(code, req.Token, req.Ready)
		case "progress":
			err = pm.Progress(code, req.Token, req.Round, req.Count, req.Hints)
		case "solve":
			out["result"], err = pm.Solve(code, req.Token, req.Round, req.Stars, req.Hints)
		case "giveup":
			err = pm.GiveUp(code, req.Token, req.Round, req.Hints)
		case "leave":
			err = pm.Leave(code, req.Token)
			req.Token = ""
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSON(w, errorStatus(err), map[string]string{"error": err.Error()})
			return
		}
		if out["snapshot"], err = pm.Snapshot(code, req.Token); err != nil {
			writeJSON(w, errorStatus(err), map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /party/{code}/events", func(w http.ResponseWriter, r *http.Request) {
		code, token := r.PathValue("code"), r.URL.Query().Get("token")
		changed, cancel, err := pm.Subscribe(code, token)
		if err != nil {
			http.Error(w, err.Error(), errorStatus(err)) // not 200: EventSource gives up
			return
		}
		defer cancel()
		rc := http.NewResponseController(w)
		rc.SetWriteDeadline(time.Time{}) // the stream outlives any server write timeout
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "retry: 2000\n\n")

		send := func() bool {
			s, err := pm.Snapshot(code, token)
			if err != nil {
				fmt.Fprint(w, "event: gone\ndata: {}\n\n")
				rc.Flush()
				return false
			}
			data, _ := json.Marshal(s)
			fmt.Fprintf(w, "data: %s\n\n", data)
			return rc.Flush() == nil
		}
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for ok := send(); ok; {
			select {
			case <-r.Context().Done():
				return
			case <-changed:
				ok = send()
			case <-heartbeat.C:
				fmt.Fprint(w, ": ping\n\n")
				ok = rc.Flush() == nil
			}
		}
	})
}

func writeParty(w http.ResponseWriter, status int, d PartyPage) {
	d.MinN, d.MaxN = MinN, MaxN
	var buf bytes.Buffer
	if err := WriteParty(&buf, d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
