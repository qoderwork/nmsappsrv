package ztp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/internal/device"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/ztp/external"
)

// validZTPSetting returns a fully-valid ZTPSetting (every validateZTPSettings
// rule passes). Tests mutate a copy to exercise each individual rule.
func validZTPSetting() *misc.ZTPSetting {
	g := func(s string) *string { return &s }
	i := func(n int) *int { return &n }
	return &misc.ZTPSetting{
		GnbIdStart:     i(1),
		GnbIdEnd:       i(100),
		GoogleAPIKey:   g("google-key"),
		PTP:            &misc.PTPSetting{ClockDomainNumber: i(24), ClockSyncMode: g("manual")},
		SpectrumSpatial: &misc.SpectrumSpatialSetting{
			GeoCodeURL:       g("http://spectrum/geocode"),
			ReverseGeoCodeURL: g("http://spectrum/reverse"),
		},
		MSAG:   &misc.ExternalEndpointSetting{URL: g("http://msag"), Username: g("u"), Password: g("p")},
		BMC:    &misc.ExternalEndpointSetting{URL: g("http://bmc"), Username: g("u"), Password: g("p")},
		NewBMC: &misc.ExternalEndpointSetting{URL: g("http://newbmc")},
		LMF:    &misc.ExternalEndpointSetting{URL: g("http://lmf"), Username: g("u"), Password: g("p")},
		GMLC:   &misc.ExternalEndpointSetting{URL: g("http://gmlc"), Username: g("u"), Password: g("p")},
	}
}

func TestValidateZTPSettingsNil(t *testing.T) {
	msg := validateZTPSettings(nil)
	require.NotNil(t, msg)
	assert.Contains(t, *msg, "missing")
}

func TestValidateZTPSettings(t *testing.T) {
	type rule struct {
		name    string
		mutate  func(*misc.ZTPSetting)
		wantMsg string // substring of expected message; "" => expect valid (nil)
	}
	rules := []rule{
		{"missing gnb start", func(s *misc.ZTPSetting) { s.GnbIdStart = nil }, "gNB ID Start is missing"},
		{"missing gnb end", func(s *misc.ZTPSetting) { s.GnbIdEnd = nil }, "gNB ID End is missing"},
		{"missing google key", func(s *misc.ZTPSetting) { s.GoogleAPIKey = nil }, "Google API Key is missing"},
		{"missing PTP", func(s *misc.ZTPSetting) { s.PTP = nil }, "PTP Setting is missing"},
		{"missing PTP clock domain", func(s *misc.ZTPSetting) { s.PTP.ClockDomainNumber = nil }, "PTP Clock Domain Number is missing"},
		{"missing PTP sync mode", func(s *misc.ZTPSetting) { s.PTP.ClockSyncMode = nil }, "PTP Clock Sync Mode is missing"},
		{"missing spectrum", func(s *misc.ZTPSetting) { s.SpectrumSpatial = nil }, "Spectrum Spatial Setting is missing"},
		{"missing spectrum geocode", func(s *misc.ZTPSetting) { s.SpectrumSpatial.GeoCodeURL = nil }, "Spectrum Spatial GeoCode URL is missing"},
		{"missing spectrum revgeo", func(s *misc.ZTPSetting) { s.SpectrumSpatial.ReverseGeoCodeURL = nil }, "Spectrum Spatial Reverse GeoCode URL is missing"},
		{"missing MSAG", func(s *misc.ZTPSetting) { s.MSAG = nil }, "MSAG Setting is missing"},
		{"missing MSAG url", func(s *misc.ZTPSetting) { s.MSAG.URL = nil }, "MSAG URL is missing"},
		{"missing MSAG user", func(s *misc.ZTPSetting) { s.MSAG.Username = nil }, "MSAG Username is missing"},
		{"missing MSAG pass", func(s *misc.ZTPSetting) { s.MSAG.Password = nil }, "MSAG Password is missing"},
		{"invalid BMC", func(s *misc.ZTPSetting) { s.BMC = nil; s.NewBMC = nil }, "BMC Setting is invalid"},
		{"invalid LMF", func(s *misc.ZTPSetting) { s.LMF = nil; s.LMF2 = nil; s.LMF3 = nil; s.LMF4 = nil }, "LMF Configuration is invalid"},
		{"missing GMLC", func(s *misc.ZTPSetting) { s.GMLC = nil }, "GMLC Setting is missing"},
		{"missing GMLC url", func(s *misc.ZTPSetting) { s.GMLC.URL = nil }, "GMLC URL is missing"},
		{"missing GMLC user", func(s *misc.ZTPSetting) { s.GMLC.Username = nil }, "GMLC Username is missing"},
		{"missing GMLC pass", func(s *misc.ZTPSetting) { s.GMLC.Password = nil }, "GMLC Password is missing"},
	}

	for _, r := range rules {
		t.Run(r.name, func(t *testing.T) {
			s := validZTPSetting()
			r.mutate(s)
			msg := validateZTPSettings(s)
			require.NotNil(t, msg, "expected rule %q to fail", r.name)
			assert.Contains(t, *msg, r.wantMsg, "rule %q", r.name)
		})
	}

	t.Run("valid", func(t *testing.T) {
		msg := validateZTPSettings(validZTPSetting())
		assert.Nil(t, msg, "expected a fully-valid setting to pass")
	})
}

func TestBmcConfigValid(t *testing.T) {
	ep := func(url, user, pass string) *misc.ExternalEndpointSetting {
		return &misc.ExternalEndpointSetting{URL: strPtr(url), Username: strPtr(user), Password: strPtr(pass)}
	}
	assert.True(t, bmcConfigValid(ep("u", "x", "y"), nil), "old BMC valid")
	assert.True(t, bmcConfigValid(nil, ep("u", "", "")), "new BMC valid (url only)")
	assert.False(t, bmcConfigValid(nil, nil), "both nil")
	assert.False(t, bmcConfigValid(ep("u", "x", ""), nil), "old BMC missing password")
	assert.False(t, bmcConfigValid(nil, ep("", "", "")), "new BMC missing url")
}

func TestLmfConfigValid(t *testing.T) {
	ep := func(url, user, pass string) *misc.ExternalEndpointSetting {
		return &misc.ExternalEndpointSetting{URL: strPtr(url), Username: strPtr(user), Password: strPtr(pass)}
	}
	assert.False(t, lmfConfigValid(nil, nil, nil, nil), "all nil")
	assert.True(t, lmfConfigValid(nil, ep("u", "x", "y"), nil, nil), "one valid LMF")
	assert.False(t, lmfConfigValid(ep("u", "", ""), nil, nil, nil), "LMF missing creds")
}

func TestParseLatLng(t *testing.T) {
	lat, lng, err := parseLatLng(strPtr("40.7"), strPtr("-74.0"))
	require.NoError(t, err)
	assert.InDelta(t, 40.7, lat, 1e-9)
	assert.InDelta(t, -74.0, lng, 1e-9)

	_, _, err = parseLatLng(nil, strPtr("-74.0"))
	assert.Error(t, err)

	_, _, err = parseLatLng(strPtr(""), strPtr("-74.0"))
	assert.Error(t, err)

	_, _, err = parseLatLng(strPtr("notafloat"), strPtr("-74.0"))
	assert.Error(t, err)
}

func TestVincentyDistance(t *testing.T) {
	// Same point → zero.
	assert.InDelta(t, 0.0, vincentyDistance(40.0, -75.0, 40.0, -75.0), 1e-6)

	// 1° of longitude at the equator ≈ 111,319 m.
	d := vincentyDistance(0, 0, 0, 1.0)
	assert.InDelta(t, 111319.0, d, 200)

	// 1° of latitude ≈ 110,574 m.
	d = vincentyDistance(0, 0, 1.0, 0)
	assert.InDelta(t, 110574.0, d, 200)

	// New York → Philadelphia (~130 km) — sanity bounds, not exact.
	d = vincentyDistance(40.7128, -74.0060, 39.9526, -75.1652)
	assert.Greater(t, d, 120000.0)
	assert.Less(t, d, 140000.0)
}

func TestGetMbPart(t *testing.T) {
	assert.Equal(t, 0x34, getMbPart("123456"))    // mid "34" → 52
	assert.Equal(t, 0xab, getMbPart("12ab34"))    // mid "ab" → 171
	assert.Equal(t, -1, getMbPart("12"))          // too short
	assert.Equal(t, -1, getMbPart("abcz"))        // mid "" → parse err
}

func TestReassembleTAC(t *testing.T) {
	// tacStart "123456", mid 52 ("34") → "123456" → 0x123456.
	v, err := reassembleTAC("123456", 0x34)
	require.NoError(t, err)
	assert.Equal(t, 0x123456, v)

	// A wider middle (8-digit TAC): "12345678", mid 0x34 → "12"+"0034"+"5678".
	v, err = reassembleTAC("12345678", 0x34)
	require.NoError(t, err)
	assert.Equal(t, 0x1200345678, v)

	_, err = reassembleTAC("12", 1)
	assert.Error(t, err, "too-short tacStart must error")
}

func TestAllocateTACNilRange(t *testing.T) {
	// With no TAC range configured, allocation is skipped and (0,0,nil) is
	// returned without touching the database — safe to call with a nil db.
	cfg := &external.ExternalConfig{} // TacStart/TacEnd nil
	finalTac, mid, err := allocateTAC(nil, cfg, "market")
	require.NoError(t, err)
	assert.Equal(t, 0, finalTac)
	assert.Equal(t, 0, mid)
}

// ---------------------------------------------------------------------------
// ZTP failure alarm (acs_url gate)
// ---------------------------------------------------------------------------

// fakeAlarmSvc is a minimal alarm.Service for unit-testing raiseZTPFailedAlarm.
// It embeds the real interface (unimplemented methods stay nil) and overrides
// only the two methods raiseZTPFailedAlarm calls.
type fakeAlarmSvc struct {
	alarm.Service
	calls     int
	lastAlarm *alarm.Alarm
	existing  *alarm.Alarm // value returned by GetByElementTypeAlarmId
}

func (f *fakeAlarmSvc) GetByElementTypeAlarmId(_ int64, _ int, _ string) (*alarm.Alarm, error) {
	return f.existing, nil
}

func (f *fakeAlarmSvc) CreateAlarm(a *alarm.Alarm) error {
	f.calls++
	f.lastAlarm = a
	return nil
}

func TestRaiseZTPFailedAlarm(t *testing.T) {
	dev := device.CpeElement{NeNeid: 42, LicenseId: intPtr(7)}

	t.Run("no existing -> creates ztp_failed alarm", func(t *testing.T) {
		fake := &fakeAlarmSvc{}
		th := &Thread{alarmSvc: fake}
		th.raiseZTPFailedAlarm(dev, "The ACS URL parameter is missing in Inform")

		require.Equal(t, 1, fake.calls, "CreateAlarm must be called once")
		a := fake.lastAlarm
		require.NotNil(t, a)
		assert.Equal(t, "ztp_failed", strOrEmpty(a.AlarmId))
		assert.Equal(t, intPtr(alarm.AlarmTypeActive), a.AlarmType)
		assert.Equal(t, "Critical", strOrEmpty(a.Severity))
		assert.Equal(t, "ZTP Alarm", strOrEmpty(a.EventType))
		assert.Equal(t, "The ACS URL parameter is missing in Inform", strOrEmpty(a.AdditionalInformation))
		assert.Equal(t, int64(42), *a.ElementId)
		assert.Equal(t, intPtr(7), a.LicenseId)
	})

	t.Run("existing same info -> dedup, no create", func(t *testing.T) {
		fake := &fakeAlarmSvc{existing: &alarm.Alarm{
			Id:                     99,
			AdditionalInformation: strPtr("The ACS URL parameter is missing in Inform"),
		}}
		th := &Thread{alarmSvc: fake}
		th.raiseZTPFailedAlarm(dev, "The ACS URL parameter is missing in Inform")
		assert.Equal(t, 0, fake.calls, "dedup must not re-create the alarm")
	})

	t.Run("skip sentinel is distinct", func(t *testing.T) {
		require.NotNil(t, errDeviceSkipped)
		assert.False(t, errors.Is(errors.New("other error"), errDeviceSkipped),
			"the skip sentinel must be distinguishable from real failures")
	})
}

