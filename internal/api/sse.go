package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	maxAbonnes    = 32               // onglets ouverts en même temps
	delaiEcriture = 10 * time.Second // une page qui ne lit plus est déconnectée
)

// Diffuseur envoie l'événement SSE « maj » à toutes les pages ouvertes.
type Diffuseur struct {
	mu      sync.Mutex
	abonnes map[chan EvenementMaj]struct{}
	ferme   bool
}

func NouveauDiffuseur() *Diffuseur {
	return &Diffuseur{abonnes: map[chan EvenementMaj]struct{}{}}
}

// Annoncer prévient les abonnés que ces tables ont changé. Un abonné trop lent perd l'événement
// plutôt que de bloquer l'import.
func (d *Diffuseur) Annoncer(tables ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := EvenementMaj{Tables: tables}
	for c := range d.abonnes {
		select {
		case c <- e:
		default:
		}
	}
}

// Fermer termine toutes les connexions (arrêt du serveur).
func (d *Diffuseur) Fermer() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ferme = true
	for c := range d.abonnes {
		close(c)
		delete(d.abonnes, c)
	}
}

func (d *Diffuseur) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, ok := w.(http.Flusher)
	if !ok {
		erreur(w, http.StatusInternalServerError, "flux non pris en charge")
		return
	}
	c := make(chan EvenementMaj, 4)
	d.mu.Lock()
	if d.ferme {
		d.mu.Unlock()
		erreur(w, http.StatusServiceUnavailable, "arrêt en cours")
		return
	}
	if len(d.abonnes) >= maxAbonnes {
		d.mu.Unlock()
		erreur(w, http.StatusTooManyRequests, "trop de pages abonnées aux mises à jour")
		return
	}
	d.abonnes[c] = struct{}{}
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		if _, ok := d.abonnes[c]; ok {
			delete(d.abonnes, c)
			close(c)
		}
		d.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	rc := http.NewResponseController(w)
	envoyer := func(texte string) bool {
		rc.SetWriteDeadline(time.Now().Add(delaiEcriture))
		if _, err := fmt.Fprint(w, texte); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !envoyer("retry: 3000\n\n") {
		return
	}

	battement := time.NewTicker(25 * time.Second)
	defer battement.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-battement.C:
			if !envoyer(": battement\n\n") {
				return
			}
		case e, ok := <-c:
			if !ok {
				return
			}
			b, _ := json.Marshal(e)
			if !envoyer(fmt.Sprintf("event: maj\ndata: %s\n\n", b)) {
				return
			}
		}
	}
}
