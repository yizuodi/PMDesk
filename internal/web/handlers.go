package web

import (
	"log"
	"net/http"
)

// serverError logs and returns a generic 500.
func serverError(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "服务器错误", http.StatusInternalServerError)
}

// badRequest returns a 400.
func badRequest(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

// notFound returns a 404.
func notFound(w http.ResponseWriter) {
	http.Error(w, "找不到页面", http.StatusNotFound)
}

// redirect is a tiny wrapper for a 303 See Other (keeps POST→GET correct).
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}
