package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLogEvent_BadRequest sprawdza, czy REST API poprawnie odrzuca
// puste lub błędne zapytania (Walidacja Danych)
func TestLogEvent_BadRequest(t *testing.T) {
	handler := &EventHandler{DB: nil}
	badPayload := []byte(`{"user_context": "test"}`)
	req, err := http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(badPayload))
	if err != nil {
		t.Fatalf("Nie udało się utworzyć żądania: %v", err)
	}
	rr := httptest.NewRecorder()
	handler.LogEvent(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Oczekiwano statusu %v (Bad Request), ale otrzymano %v", http.StatusBadRequest, status)
	}
	expectedErrorFragment := "Brakujące ad_id lub niepoprawny event_type"
	if !bytes.Contains(rr.Body.Bytes(), []byte(expectedErrorFragment)) {
		t.Errorf("Oczekiwano komunikatu błędu zawierającego: '%s', ale otrzymano: '%s'", expectedErrorFragment, rr.Body.String())
	}
}

// TestLogEvent_InvalidJSON sprawdza, czy API poprawnie wyłapuje uszkodzony,
// nieparsowalny ciąg znaków (Invalid JSON) chroniąc serwer przed awarią (panic)
func TestLogEvent_InvalidJSON(t *testing.T) {
	handler := &EventHandler{DB: nil}
	brokenJSON := []byte(`{"ad_id": "123", "event_type": impression`)

	req, err := http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(brokenJSON))
	if err != nil {
		t.Fatalf("Nie udało się utworzyć żądania: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.LogEvent(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Oczekiwano statusu %v, ale otrzymano %v", http.StatusBadRequest, status)
	}

	expectedErrorFragment := "Niepoprawny format JSON"
	if !bytes.Contains(rr.Body.Bytes(), []byte(expectedErrorFragment)) {
		t.Errorf("Oczekiwano komunikatu o złym formacie JSON, ale otrzymano: '%s'", rr.Body.String())
	}
}
