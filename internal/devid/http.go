package devid

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// EnrollStartRequest is the JSON body for POST /latest/devid/enroll/start.
type EnrollStartRequest struct {
	RequestB64   string `json:"request_b64"`
	SignatureB64 string `json:"signature_b64"`
}

// EnrollStartResponse is returned from enroll/start.
type EnrollStartResponse struct {
	SessionID         string `json:"session_id"`
	CredentialBlobB64 string `json:"credential_blob_b64"`
	SecretB64         string `json:"secret_b64"`
}

// EnrollFinishRequest is the JSON body for POST /latest/devid/enroll/finish.
type EnrollFinishRequest struct {
	SessionID            string `json:"session_id"`
	ChallengeResponseB64 string `json:"challenge_response_b64"`
}

// EnrollFinishResponse is returned from enroll/finish.
type EnrollFinishResponse struct {
	DevIDCertPEM string `json:"devid_cert_pem"`
}

// Register mounts DevID enroll handlers on mux.
func Register(mux *http.ServeMux, e *Enroller) {
	mux.HandleFunc("POST /latest/devid/enroll/start", e.HandleStart)
	mux.HandleFunc("POST /latest/devid/enroll/finish", e.HandleFinish)
}

func writeEnrollAuthError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	if isUnauthorized(err) {
		code = http.StatusUnauthorized
	}
	log.Printf("devid enroll auth: %v", err)
	http.Error(w, err.Error(), code)
}

// HandleStart verifies MAC inventory identity + EK cert header, then the CSR.
func (e *Enroller) HandleStart(w http.ResponseWriter, r *http.Request) {
	var req EnrollStartRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	requestData, err := base64.StdEncoding.DecodeString(req.RequestB64)
	if err != nil {
		http.Error(w, "invalid request_b64", http.StatusBadRequest)
		return
	}
	sig, err := base64.StdEncoding.DecodeString(req.SignatureB64)
	if err != nil {
		http.Error(w, "invalid signature_b64", http.StatusBadRequest)
		return
	}

	// Peek CSR EK so the auth header can be bound to the signing request.
	var sr SigningRequest
	if err := sr.UnmarshalBinary(requestData); err != nil {
		http.Error(w, "invalid signing request", http.StatusBadRequest)
		return
	}

	vmid, ekCert, err := e.AuthenticateEnrollCaller(r, sr.EndorsementCertificate, "")
	if err != nil {
		writeEnrollAuthError(w, err)
		return
	}
	ekFP := EKFingerprint(ekCert)

	result, err := e.Start(requestData, sig, vmid, vmid, ekFP)
	if err != nil {
		log.Printf("devid enroll/start: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(EnrollStartResponse{
		SessionID:         result.SessionID,
		CredentialBlobB64: base64.StdEncoding.EncodeToString(result.CredentialBlob),
		SecretB64:         base64.StdEncoding.EncodeToString(result.Secret),
	})
}

// HandleFinish re-authenticates MAC + EK, then completes enrollment.
func (e *Enroller) HandleFinish(w http.ResponseWriter, r *http.Request) {
	var req EnrollFinishRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := base64.StdEncoding.DecodeString(req.ChallengeResponseB64)
	if err != nil {
		http.Error(w, "invalid challenge_response_b64", http.StatusBadRequest)
		return
	}

	vmid, ekCert, err := e.AuthenticateEnrollCaller(r, nil, "")
	if err != nil {
		writeEnrollAuthError(w, err)
		return
	}
	ekFP := EKFingerprint(ekCert)

	pemBytes, err := e.Finish(req.SessionID, resp, vmid, ekFP)
	if err != nil {
		log.Printf("devid enroll/finish: %v", err)
		status := http.StatusBadRequest
		if err.Error() == "VM identity does not match enroll session" ||
			err.Error() == "EK certificate does not match enroll session" {
			status = http.StatusUnauthorized
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(EnrollFinishResponse{
		DevIDCertPEM: string(pemBytes),
	})
}
