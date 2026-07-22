package external

// T-Platform alert outcall (Proof-of-Possession signed OAuth2 + alert POST).
//
// This is NOT a ZTP registration registrar. In Java it is an event-driven
// component (TPlatformHelper, triggered by TplatformMessageConsumer from a
// RabbitMQ notification) that pushes operational alarms to the T-Mobile
// T-Platform alert-management API. The ZTP orchestrator itself never calls it
// during cell registration — it is wired as a separate notify path, so it is
// kept out of Registry.Registrars().
//
// The wire protocol mirrors Java exactly:
//   - refreshToken: POST /oauth2/v1/tokens with a "cnf" claim carrying the
//     public key, authenticated by a PoP JWT (authToken="test") and HTTP Basic
//     client_credentials; the response yields id_token + access_token, cached
//     for 50 minutes.
//   - notify: POST /partner-interactions/alert-management/v1/alerts with a PoP
//     JWT (authToken = Base64(clientId:secret)), Authorization = "Basic  " +
//     access_token, X-Authorization = PoP, X-Auth-Originator = id_token,
//     interaction-id = UUID. On HTTP 200 the per-system "t_platform_unavailable"
//     alarm is cleared; on exhaustion of retries it is raised.
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/misc"
	"nmsappsrv/pkg/logger"
)

// TPlatformAlertRequest mirrors Java TPlatformHelper.AlertRequest — the alert
// payload POSTed to the T-Platform alert-management endpoint.
type TPlatformAlertRequest struct {
	Type                string            `json:"type"`
	SubType             string            `json:"subType"`
	Severity            string            `json:"severity"`
	Description         string            `json:"description"`
	OccurrenceDate      string            `json:"occurrenceDate"`
	Attributes          map[string]string `json:"attributes"`
	CustomerIdentifiers map[string]string `json:"customerIdentifiers"`
}

// tPlatformToken mirrors Java TPlatformTokenDTO {id_token, access_token}.
type tPlatformToken struct {
	IDToken    string `json:"id_token"`
	AccessToken string `json:"access_token"`
}

const (
	tPlatformTokenURIPath = "/oauth2/v1/tokens"
	tPlatformAlertURIPath = "/partner-interactions/alert-management/v1/alerts"

	// tPlatformAlarmID is the system-level alarm Java raises/clears on T-Platform
	// outcall failure/success. It is a single alarm (not per device), queried
	// by (alarm_type=ACTIVE, alarm_id).
	tPlatformAlarmID   = "t_platform_unavailable"
	tPlatformAlarmType = alarm.AlarmTypeActive // 1 (ACTIVE)

	// tPlatformTokenTTL mirrors Java's 50*60*1000 ms refresh window.
	tPlatformTokenTTL = 50 * time.Minute
)

// TPlatformClient implements the T-Platform alert outcall. The OAuth2 token
// cache (id_token/access_token) survives across Notify calls — Java keeps it on
// the singleton @Component — so a single client instance should be reused. It
// is safe for concurrent use (the token cache is mutex-guarded).
type TPlatformClient struct {
	cfg        *misc.TPlatformSetting
	transport  Transport
	privKeyPEM string
	pubKeyPEM  string
	alarmSvc   alarm.Service

	mu          sync.Mutex
	idToken     string
	accessToken string
	lastFresh   time.Time
}

// NewTPlatformClient builds a T-Platform client. cfg (the endpoint config) is
// supplied later via SetConfig (the caller reads it from the live ZTPSetting);
// tr carries the PoP key PEMs and the shared (GMLC-cert) transport Java uses
// for T-Platform; alarmSvc raises/clears the t_platform_unavailable alarm.
func NewTPlatformClient(tr *Transports, alarmSvc alarm.Service) *TPlatformClient {
	c := &TPlatformClient{
		transport: NotImplementedTransport{},
		alarmSvc:  alarmSvc,
	}
	if tr != nil {
		c.transport = tr.Shared
		c.privKeyPEM = tr.TPrivateKeyPEM
		c.pubKeyPEM = tr.TPublicKeyPEM
	}
	return c
}

// SetConfig updates the endpoint credentials. The token cache is preserved
// across configuration changes (matching Java's static fields).
func (c *TPlatformClient) SetConfig(cfg *misc.TPlatformSetting) {
	if cfg == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
}

// Enabled reports whether T-Platform alerting is configured (url + clientId +
// secret), mirroring Java's notify guard.
func (c *TPlatformClient) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg != nil && strOrEmpty(c.cfg.URL) != "" &&
		strOrEmpty(c.cfg.ClientID) != "" && strOrEmpty(c.cfg.Secret) != ""
}

// Notify sends an alert to T-Platform.
//
//   - (false, nil) when T-Platform is not configured — Java simply returns
//     without raising an alarm.
//   - (true, nil) when the alert was accepted (HTTP 200); the
//     t_platform_unavailable alarm is cleared.
//   - (false, err) when every retry was rejected/errored; the
//     t_platform_unavailable alarm is raised. The returned error is
//     informational — callers must NOT treat it as fatal (the alarm already
//     reflects the outage), mirroring Java's swallow-and-alarm behaviour.
func (c *TPlatformClient) Notify(ctx context.Context, req *TPlatformAlertRequest) (bool, error) {
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	if cfg == nil || !c.enabledLocked(cfg) {
		return false, nil
	}
	// Java sets occurrenceDate to yyyy-MM-dd'T'HH:mm:ss'Z' (UTC).
	if req != nil {
		req.OccurrenceDate = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}

	retry := 1
	if cfg.RetryTimes != nil && *cfg.RetryTimes > 0 {
		retry = *cfg.RetryTimes
	}

	var lastErr error
	for i := 0; i < retry; i++ {
		if err := c.ensureToken(ctx); err != nil {
			lastErr = err
			continue // refresh will be retried on the next iteration
		}
		ok, err := c.postAlert(ctx, cfg, req)
		if err != nil {
			lastErr = err
			continue
		}
		if ok {
			c.clearTplatformAlarm()
			return true, nil
		}
		lastErr = fmt.Errorf("t-platform: alert rejected (non-200)")
	}
	c.raiseTplatformAlarm()
	return false, lastErr
}

// enabledLocked reports configuration without re-locking (cfg already read).
func (c *TPlatformClient) enabledLocked(cfg *misc.TPlatformSetting) bool {
	return strOrEmpty(cfg.URL) != "" &&
		strOrEmpty(cfg.ClientID) != "" && strOrEmpty(cfg.Secret) != ""
}

// ensureToken refreshes the OAuth2 token when it is missing or older than the
// 50-minute TTL. The mutex is released during the network call to avoid
// blocking concurrent callers; a refresh failure nulls the cached tokens
// (matching Java getTokenFailedAfter).
func (c *TPlatformClient) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	valid := c.idToken != "" && c.accessToken != "" && time.Since(c.lastFresh) < tPlatformTokenTTL
	c.mu.Unlock()
	if valid {
		return nil
	}
	idTok, accTok, err := c.refreshToken(ctx)
	if err != nil {
		c.mu.Lock()
		c.idToken = ""
		c.accessToken = ""
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	c.idToken = idTok
	c.accessToken = accTok
	c.lastFresh = time.Now()
	c.mu.Unlock()
	return nil
}

// refreshToken obtains id_token + access_token from the T-Platform OAuth2
// endpoint, mirroring Java TPlatformHelper.refreshToken.
func (c *TPlatformClient) refreshToken(ctx context.Context) (string, string, error) {
	c.mu.Lock()
	cfg := c.cfg
	privKey := c.privKeyPEM
	pubKey := c.pubKeyPEM
	c.mu.Unlock()
	if privKey == "" {
		return "", "", fmt.Errorf("t-platform: private key PEM not loaded")
	}
	// cnf claim = the public key content.
	data := map[string]string{"cnf": pubKey}
	body, _ := json.Marshal(data)
	// Java passes authToken="test" for the token-endpoint PoP.
	pop, err := BuildPopToken(privKey, string(body), "test", tPlatformTokenURIPath)
	if err != nil {
		return "", "", fmt.Errorf("t-platform: build pop token (token endpoint): %w", err)
	}
	base := strings.TrimRight(strOrEmpty(cfg.URL), "/") + tPlatformTokenURIPath
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method: "POST",
		URL:    base,
		Headers: map[string]string{
			"Content-Type":    "application/json",
			"grant-type":      "client_credentials",
			"Authorization":   "Basic  " + base64.StdEncoding.EncodeToString([]byte(strOrEmpty(cfg.ClientID)+":"+strOrEmpty(cfg.Secret))),
			"X-Authorization": pop,
		},
		Body: body,
	})
	if err != nil {
		return "", "", fmt.Errorf("t-platform: token request: %w", err)
	}
	var tok tPlatformToken
	if err := json.Unmarshal(resp.Body, &tok); err != nil {
		return "", "", fmt.Errorf("t-platform: parse token response: %w", err)
	}
	if tok.IDToken == "" || tok.AccessToken == "" {
		return "", "", fmt.Errorf("t-platform: token response missing id_token/access_token (status %d)", resp.StatusCode)
	}
	return tok.IDToken, tok.AccessToken, nil
}

// postAlert POSTs the alert to the T-Platform alert-management endpoint,
// mirroring Java TPlatformHelper.notify's inner send. Returns accepted=true on
// HTTP 200.
func (c *TPlatformClient) postAlert(ctx context.Context, cfg *misc.TPlatformSetting, req *TPlatformAlertRequest) (bool, error) {
	c.mu.Lock()
	privKey := c.privKeyPEM
	idTok := c.idToken
	accTok := c.accessToken
	c.mu.Unlock()
	if privKey == "" {
		return false, fmt.Errorf("t-platform: private key PEM not loaded")
	}
	body, _ := json.Marshal(req)
	// Java passes authToken = Base64(clientId:secret) for the alert-endpoint PoP.
	authToken := base64.StdEncoding.EncodeToString([]byte(strOrEmpty(cfg.ClientID) + ":" + strOrEmpty(cfg.Secret)))
	pop, err := BuildPopToken(privKey, string(body), authToken, tPlatformAlertURIPath)
	if err != nil {
		return false, fmt.Errorf("t-platform: build pop token (alert): %w", err)
	}
	base := strings.TrimRight(strOrEmpty(cfg.URL), "/") + tPlatformAlertURIPath
	resp, err := c.transport.RoundTrip(ctx, &TransportRequest{
		Method: "POST",
		URL:    base,
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"grant-type":        "client_credentials",
			"Authorization":     "Basic  " + accTok,
			"X-Authorization":   pop,
			"X-Auth-Originator": idTok,
			"interaction-id":    uuid.New().String(),
		},
		Body: body,
	})
	if err != nil {
		return false, fmt.Errorf("t-platform: alert request: %w", err)
	}
	return resp.StatusCode == 200, nil
}

// clearTplatformAlarm clears the single system-level t_platform_unavailable
// alarm if it is currently active, mirroring Java clearTplatformAlarm.
func (c *TPlatformClient) clearTplatformAlarm() {
	if c.alarmSvc == nil {
		return
	}
	existing, err := c.alarmSvc.GetByAlarmId(tPlatformAlarmType, tPlatformAlarmID)
	if err != nil {
		logger.Warnf("t-platform: query existing alarm before clear failed: %v", err)
		return
	}
	if existing == nil {
		return
	}
	if err := c.alarmSvc.ClearAlarm(existing.Id); err != nil {
		logger.Warnf("t-platform: clear t_platform_unavailable alarm failed: %v", err)
	}
}

// raiseTplatformAlarm raises the single system-level t_platform_unavailable
// alarm if it is not already active, mirroring Java generateTplatformAlarm.
// Field values come from the alarm_library seed (V1_97__init.sql).
func (c *TPlatformClient) raiseTplatformAlarm() {
	if c.alarmSvc == nil {
		return
	}
	existing, err := c.alarmSvc.GetByAlarmId(tPlatformAlarmType, tPlatformAlarmID)
	if err != nil {
		logger.Warnf("t-platform: query existing alarm before raise failed: %v", err)
		return
	}
	if existing != nil {
		return // already raised
	}
	now := time.Now()
	status := alarm.AlarmStatusActiveUnconfirmed
	aType := alarm.AlarmTypeActive
	al := &alarm.Alarm{
		AlarmId:         strPtr(tPlatformAlarmID),
		AlarmIdentifier: strPtr(strconv.FormatInt(now.UnixMilli(), 10)),
		AlarmSource:     strPtr("OMC"),
		AlarmStatus:     &status,
		AlarmType:       &aType,
		EventTime:       &now,
		EventType:       strPtr("ZTP Alarm"),
		NetworkElement:  strPtr("DN='OMC'"),
		ProbableCause:   strPtr("T-Platform is unreachable"),
		Severity:        strPtr("Critical"),
		SpecificProblem: strPtr("T-Platform is unreachable"),
		UpdateTime:      &now,
		// ElementId / TenantId left nil: a system-level alarm, not per device.
	}
	if err := c.alarmSvc.CreateAlarm(al); err != nil {
		logger.Warnf("t-platform: raise t_platform_unavailable alarm failed: %v", err)
	}
}
