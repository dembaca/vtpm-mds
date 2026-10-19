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

// ClientOptions configures ClientEnrollOrRenew.
type ClientOptions struct {
	MDSBaseURL   string
	PlatformCN   string
	OutDir       string
	SpireDataDir string // empty skips SVID cache clear / try-restart
	Now          func() time.Time
}

// ClientEnroll performs the full DevID enrollment against an MDS base URL.
// The guest authenticates with:
//  1. Source MAC → inventory VM identity (server ConnContext / ARP)
//  2. TPM EK certificate in HeaderEKCert on every enroll request
//
// It writes devid.crt.pem, devid.priv.blob, and devid.pub.blob under outDir.
// Prefer ClientEnrollOrRenew for packaged renewals.
func ClientEnroll(rw io.ReadWriter, mdsBaseURL, platformCN, outDir string) error {
	return ClientEnrollOrRenew(rw, ClientOptions{
		MDSBaseURL: mdsBaseURL,
		PlatformCN: platformCN,
		OutDir:     outDir,
	})
}

// ClientEnrollOrRenew renews when due (or materials are missing); otherwise
// leaves files unchanged. On a PEM change it clears the SPIRE agent SVID
// cache (when SpireDataDir is set) and try-restarts spire-agent.
func ClientEnrollOrRenew(rw io.ReadWriter, opts ClientOptions) error {
	if opts.MDSBaseURL == "" {
		return fmt.Errorf("mds base URL required")
	}
	if opts.OutDir == "" {
		return fmt.Errorf("output directory required")
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()

	certPath := filepath.Join(opts.OutDir, "devid.crt.pem")
	privPath := filepath.Join(opts.OutDir, "devid.priv.blob")
	pubPath := filepath.Join(opts.OutDir, "devid.pub.blob")

	existing, err := LoadCertPEMFile(certPath)
	if err != nil {
		return err
	}
	if existing != nil && !RenewDue(existing, now) {
		return nil
	}

	if err := os.MkdirAll(opts.OutDir, 0o700); err != nil {
		return err
	}

	var oldPEM []byte
	if existing != nil {
		oldPEM, _ = os.ReadFile(certPath)
	}

	pubBlob, _ := os.ReadFile(pubPath)
	privBlob, _ := os.ReadFile(privPath)
	// Reuse only when both blobs load; otherwise create a new key.
	reusePub, reusePriv := pubBlob, privBlob
	if len(reusePub) == 0 || len(reusePriv) == 0 {
		reusePub, reusePriv = nil, nil
	}

	factory := DefaultKeyFactory()
	sr, res, err := CreateSigningRequestWithDevID(rw, factory, opts.PlatformCN, reusePub, reusePriv)
	if err != nil {
		// Blobs unusable under the SPIRE H-1 parent — fall back to a new key.
		if reusePub != nil {
			sr, res, err = CreateSigningRequestWithDevID(rw, factory, opts.PlatformCN, nil, nil)
		}
		if err != nil {
			return err
		}
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
	startResp, err := httpPostJSON(opts.MDSBaseURL+"/latest/devid/enroll/start", startBody, ekAuth)
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
	finishResp, err := httpPostJSON(opts.MDSBaseURL+"/latest/devid/enroll/finish", finishBody, ekAuth)
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

	newPEM := []byte(finish.DevIDCertPEM)
	if err := os.WriteFile(certPath, newPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(privPath, res.DevID.PrivateBlob, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, res.DevID.PublicBlob, 0o600); err != nil {
		return err
	}

	if !bytes.Equal(oldPEM, newPEM) {
		if err := ClearSPIREAgentSVIDCache(opts.SpireDataDir); err != nil {
			return err
		}
		if opts.SpireDataDir != "" {
			TryRestartSPIREAgent()
		}
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
