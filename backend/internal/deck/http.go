package deck

import (
	"net/http"

	"github.com/trick77/ship/kit/httpx"

	"github.com/trick77/reworm/internal/api"
)

// Mount adds GET /api/decks.
func (r *Repo) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/decks", r.handleDecks)
}

func (r *Repo) handleDecks(w http.ResponseWriter, req *http.Request) {
	d, err := r.Decks(req.Context())
	if err != nil {
		api.WriteError(w, req, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}
