package devid

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-tpm/legacy/tpm2"
)

// ClientEnroll performs the full DevID enrollment against an MDS base URL.
// The guest authenticates with:
//  1. Source MAC → inventory VM identity (server ConnContext / ARP)
//  2. TPM EK certificate in HeaderEKCert on every enroll request
//
// It writes devid.crt.pem, devid.priv.blob, and devid.pub.blob under outDir.
func ClientEnroll(rw io.ReadWriter, mdsBaseURL, platformCN, outDir string) error {
	if mdsBaseURL == "" {
		return fmt.Errorf("mds base URL required")
	}
	if outDir == "" {
		return fmt.Errorf("output directory required")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return err
	}

	factory := DefaultKeyFactory()
	sr, res, err := CreateSigningRequest(rw, factory, platformCN)
	if err != nil {
		return err
	}
	defer res.Flush()

	if sr.EndorsementCertificate == nil {
		return fmt.Errorf("TPM EK certificate required for enroll authentication")
	}
	ekAuth := EncodeEKCertHeader(sr.EndorsementCertificate)

	requestData, err := sr.MarshalBinary()
	if err != nil {
		return err
	}
	sig, err := HashAndSign(rw, tpm2.HandleOwner, res.DevID.Handle, requestData)
	if err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	startBody, _ := json.Marshal(EnrollStartRequest{
		RequestB64:   base64.StdEncoding.EncodeToString(requestData),
		SignatureB64: base64.StdEncoding.EncodeToString(sig),
	})
	startResp, err := httpPostJSON(mdsBaseURL+"/latest/devid/enroll/start", startBody, ekAuth)
	if err != nil {
		return fmt.Errorf("enroll/start: %w", err)
	}
	var start EnrollStartResponse
	if err := json.Unmarshal(startResp, &start); err != nil {
		return fmt.Errorf("decode start response: %w", err)
	}
	cred, err := base64.StdEncoding.DecodeString(start.CredentialBlobB64)
	if err != nil {
		return err
	}
	secret, err := base64.StdEncoding.DecodeString(start.SecretB64)
	if err != nil {
		return err
	}

	challengeResp, err := res.Activate(cred, secret)
	if err != nil {
		return fmt.Errorf("activate credential: %w", err)
	}

	finishBody, _ := json.Marshal(EnrollFinishRequest{
		SessionID:            start.SessionID,
		ChallengeResponseB64: base64.StdEncoding.EncodeToString(challengeResp),
	})
	finishResp, err := httpPostJSON(mdsBaseURL+"/latest/devid/enroll/finish", finishBody, ekAuth)
	if err != nil {
		return fmt.Errorf("enroll/finish: %w", err)
	}
	var finish EnrollFinishResponse
	if err := json.Unmarshal(finishResp, &finish); err != nil {
		return fmt.Errorf("decode finish response: %w", err)
	}
	if finish.DevIDCertPEM == "" {
		return fmt.Errorf("empty DevID certificate in response")
	}

	if err := os.WriteFile(filepath.Join(outDir, "devid.crt.pem"), []byte(finish.DevIDCertPEM), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "devid.priv.blob"), res.DevID.PrivateBlob, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "devid.pub.blob"), res.DevID.PublicBlob, 0o600); err != nil {
		return err
	}
	return nil
}

func httpPostJSON(url string, body []byte, ekCertHeader string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if ekCertHeader != "" {
		req.Header.Set(HeaderEKCert, ekCertHeader)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
