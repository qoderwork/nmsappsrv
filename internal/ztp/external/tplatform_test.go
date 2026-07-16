package external

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/misc"
)

// ---------------------------------------------------------------------------
// Test RSA key helper
// ---------------------------------------------------------------------------

func genRSAKeyPEM(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return priv, string(pemBytes)
}

// ---------------------------------------------------------------------------
// BuildPopToken
// ---------------------------------------------------------------------------

func TestBuildPopToken(t *testing.T) {
	priv, privPEM := genRSAKeyPEM(t)
	body := `{"type":"TEST","description":"hello"}`
	authToken := base64.StdEncoding.EncodeToString([]byte("client:secret"))
	uri := "/partner-interactions/alert-management/v1/alerts"

	tok, err := BuildPopToken(privPEM, body, authToken, uri)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	// Parse + verify signature with the public key.
	parsed, err := jwt.ParseWithClaims(tok, &popTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &priv.PublicKey, nil
	})
	require.NoError(t, err)

	claims, ok := parsed.Claims.(*popTokenClaims)
	require.True(t, ok)

	// ehts must list the keys in LinkedHashMap order, semicolon-joined.
	assert.Equal(t, "Content-Type;Authorization;uri;http-method;body", claims.Ehts)

	// v == "1".
	assert.Equal(t, "1", claims.V)

	// edts = SHA-256(Base64 URL-safe no-pad) of the concatenated ehts VALUES.
	edtsSrc := "application/json" + authToken + uri + "POST" + body
	assert.Equal(t, edtsHash(edtsSrc), claims.Edts)

	// jti is a valid UUID.
	_, err = uuid.Parse(claims.RegisteredClaims.ID)
	assert.NoError(t, err)

	// iat/exp → 2-minute validity.
	require.NotNil(t, claims.RegisteredClaims.ExpiresAt)
	require.NotNil(t, claims.RegisteredClaims.IssuedAt)
	assert.Equal(t, int64(120), int64(claims.RegisteredClaims.ExpiresAt.Time.Sub(claims.RegisteredClaims.IssuedAt.Time).Seconds()))
}

func TestBuildPopTokenBadKey(t *testing.T) {
	_, err := BuildPopToken("not-a-pem", "body", "auth", "/uri")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// fake alarm service for T-Platform tests
// ---------------------------------------------------------------------------

type fakeTPAlarmSvc struct {
	alarm.Service
	mu      sync.Mutex
	byID    map[string]*alarm.Alarm
	cleared []int64
	created []*alarm.Alarm
}

func newFakeTPAlarmSvc() *fakeTPAlarmSvc {
	return &fakeTPAlarmSvc{byID: map[string]*alarm.Alarm{}}
}

func (f *fakeTPAlarmSvc) GetByAlarmId(alarmType int, alarmId string) (*alarm.Alarm, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byID[fmt.Sprintf("%d:%s", alarmType, alarmId)], nil
}

func (f *fakeTPAlarmSvc) ClearAlarm(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared = append(f.cleared, id)
	return nil
}

func (f *fakeTPAlarmSvc) CreateAlarm(a *alarm.Alarm) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a.Id = int64(len(f.created) + 1)
	f.created = append(f.created, a)
	f.byID[fmt.Sprintf("%d:%s", *a.AlarmType, *a.AlarmId)] = a
	return nil
}

// ---------------------------------------------------------------------------
// TPlatformClient.Notify
// ---------------------------------------------------------------------------

func newTestTPlatformClient(t *testing.T, ft FuncTransport, privPEM, pubPEM string, svc alarm.Service) *TPlatformClient {
	return NewTPlatformClient(&Transports{Shared: ft, TPrivateKeyPEM: privPEM, TPublicKeyPEM: pubPEM}, svc)
}

func TestTPlatformNotifyOK(t *testing.T) {
	priv, privPEM := genRSAKeyPEM(t)
	pubPEM := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END PUBLIC KEY-----"

	var captured *TransportRequest
	var mu sync.Mutex
	ft := FuncTransport(func(_ context.Context, req *TransportRequest) (*TransportResponse, error) {
		mu.Lock()
		captured = req
		mu.Unlock()
		if strings.HasSuffix(req.URL, tPlatformTokenURIPath) {
			return &TransportResponse{StatusCode: 200, Body: []byte(`{"id_token":"ID","access_token":"ACC"}`)}, nil
		}
		return &TransportResponse{StatusCode: 200, Body: []byte(`{"status":"ok"}`)}, nil
	})

	svc := newFakeTPAlarmSvc()
	client := newTestTPlatformClient(t, ft, privPEM, pubPEM, svc)
	u := "https://tplatform.example.com/"
	cid := "client"
	sec := "secret"
	client.SetConfig(&misc.TPlatformSetting{URL: &u, ClientID: &cid, Secret: &sec})

	accepted, err := client.Notify(context.Background(), &TPlatformAlertRequest{
		Type: "E911", Severity: "Critical", Description: "down",
	})
	assert.NoError(t, err)
	assert.True(t, accepted)

	// No alarm raised; nothing cleared (none existed).
	assert.Len(t, svc.created, 0)
	assert.Len(t, svc.cleared, 0)

	// The alert request carried the right headers.
	mu.Lock()
	req := captured
	mu.Unlock()
	require.NotNil(t, req)
	assert.Equal(t, "Basic  ACC", req.Headers["Authorization"])
	assert.Equal(t, "ID", req.Headers["X-Auth-Originator"])
	assert.Equal(t, "client_credentials", req.Headers["grant-type"])
	assert.NotEmpty(t, req.Headers["X-Authorization"])
	assert.NotEmpty(t, req.Headers["interaction-id"])

	// The X-Authorization PoP token parses, is signed RS256, and its edts
	// matches the alert body (proving the ehts proof-of-possession is correct).
	popParsed, perr := jwt.ParseWithClaims(req.Headers["X-Authorization"], &popTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected alg")
		}
		return &priv.PublicKey, nil
	})
	require.NoError(t, perr)
	pc := popParsed.Claims.(*popTokenClaims)
	assert.Equal(t, "Content-Type;Authorization;uri;http-method;body", pc.Ehts)
	authToken := base64.StdEncoding.EncodeToString([]byte("client:secret"))
	edtsSrc := "application/json" + authToken + tPlatformAlertURIPath + "POST" + string(req.Body)
	assert.Equal(t, edtsHash(edtsSrc), pc.Edts)
}

func TestTPlatformNotifyClearsExistingAlarm(t *testing.T) {
	_, privPEM := genRSAKeyPEM(t)
	pubPEM := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END PUBLIC KEY-----"

	ft := FuncTransport(func(_ context.Context, req *TransportRequest) (*TransportResponse, error) {
		if strings.HasSuffix(req.URL, tPlatformTokenURIPath) {
			return &TransportResponse{StatusCode: 200, Body: []byte(`{"id_token":"ID","access_token":"ACC"}`)}, nil
		}
		return &TransportResponse{StatusCode: 200}, nil
	})
	svc := newFakeTPAlarmSvc()
	// Pre-seed an active t_platform_unavailable alarm.
	active := alarm.AlarmTypeActive
	svc.created = append(svc.created, &alarm.Alarm{Id: 7, AlarmType: &active, AlarmId: strPtr(tPlatformAlarmID)})
	svc.byID[fmt.Sprintf("%d:%s", active, tPlatformAlarmID)] = svc.created[0]

	client := newTestTPlatformClient(t, ft, privPEM, pubPEM, svc)
	u := "https://tplatform.example.com/"
	cid := "client"
	sec := "secret"
	client.SetConfig(&misc.TPlatformSetting{URL: &u, ClientID: &cid, Secret: &sec})

	accepted, err := client.Notify(context.Background(), &TPlatformAlertRequest{Type: "E911"})
	assert.NoError(t, err)
	assert.True(t, accepted)
	assert.Len(t, svc.cleared, 1)
	assert.Equal(t, int64(7), svc.cleared[0])
}

func TestTPlatformNotifyFailureRaisesAlarm(t *testing.T) {
	_, privPEM := genRSAKeyPEM(t)
	pubPEM := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END PUBLIC KEY-----"

	ft := FuncTransport(func(_ context.Context, req *TransportRequest) (*TransportResponse, error) {
		if strings.HasSuffix(req.URL, tPlatformTokenURIPath) {
			return &TransportResponse{StatusCode: 200, Body: []byte(`{"id_token":"ID","access_token":"ACC"}`)}, nil
		}
		return &TransportResponse{StatusCode: 500, Body: []byte(`{"error":"boom"}`)}, nil
	})
	svc := newFakeTPAlarmSvc()
	client := newTestTPlatformClient(t, ft, privPEM, pubPEM, svc)
	u := "https://tplatform.example.com/"
	cid := "client"
	sec := "secret"
	client.SetConfig(&misc.TPlatformSetting{URL: &u, ClientID: &cid, Secret: &sec})

	accepted, err := client.Notify(context.Background(), &TPlatformAlertRequest{Type: "E911"})
	assert.Error(t, err)
	assert.False(t, accepted)
	require.Len(t, svc.created, 1)
	assert.Equal(t, tPlatformAlarmID, *svc.created[0].AlarmId)
	assert.Equal(t, "Critical", *svc.created[0].Severity)
	assert.Equal(t, "T-Platform is unreachable", *svc.created[0].ProbableCause)
}

func TestTPlatformNotifyDisabled(t *testing.T) {
	_, privPEM := genRSAKeyPEM(t)
	ft := FuncTransport(func(_ context.Context, req *TransportRequest) (*TransportResponse, error) {
		return &TransportResponse{StatusCode: 200}, nil
	})
	svc := newFakeTPAlarmSvc()
	client := newTestTPlatformClient(t, ft, privPEM, "", svc)
	// No config set → disabled → (false, nil), no notify, no alarm.
	accepted, err := client.Notify(context.Background(), &TPlatformAlertRequest{Type: "E911"})
	assert.NoError(t, err)
	assert.False(t, accepted)
	assert.Len(t, svc.created, 0)
}

func TestTPlatformNotifyTokenRefreshFails(t *testing.T) {
	_, privPEM := genRSAKeyPEM(t)
	pubPEM := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END PUBLIC KEY-----"

	ft := FuncTransport(func(_ context.Context, req *TransportRequest) (*TransportResponse, error) {
		if strings.HasSuffix(req.URL, tPlatformTokenURIPath) {
			return &TransportResponse{StatusCode: 401, Body: []byte(`{"error":"unauthorized"}`)}, nil
		}
		return &TransportResponse{StatusCode: 200}, nil
	})
	svc := newFakeTPAlarmSvc()
	client := newTestTPlatformClient(t, ft, privPEM, pubPEM, svc)
	u := "https://tplatform.example.com/"
	cid := "client"
	sec := "secret"
	client.SetConfig(&misc.TPlatformSetting{URL: &u, ClientID: &cid, Secret: &sec})

	accepted, err := client.Notify(context.Background(), &TPlatformAlertRequest{Type: "E911"})
	assert.Error(t, err)
	assert.False(t, accepted)
	// Token refresh failed → alert never sent → alarm raised.
	assert.Len(t, svc.created, 1)
}
