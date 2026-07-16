package ztp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CORE_NR_FEMTO CSV parsing
// ---------------------------------------------------------------------------

func sampleCoreCSV() string {
	// 18 columns, header in arbitrary case/order to exercise name matching.
	hdr := "market,timezone,dst,market_abbr,tac_start,tac_end,gnbid_start,gnbid_end," +
		"pci,plmn,final_oam_tunnel_fqdn,final_core_tunnel_fqdn,amf_pool_default_ipv4_ip," +
		"amf_pool_default_ipv6_ip,amf_pool_primary_ipv4_ips,amf_pool_secondary_ipv4_ips," +
		"amf_pool_primary_ipv6_ips,amf_pool_secondary_ipv6_ips"
	rows := []string{
		hdr,
		"NYC,EST,N,nyc,100,200,1000,2000,1|2|3,310-260,oam.example.com,core.example.com,10.0.0.1,,10.0.1.1;10.0.1.2,,10.0.2.1;,",
		"LAX,PST,Y,lax,300,400,3000,4000,4|5,311-480,oam2.example.com,core2.example.com,10.1.0.1,,10.1.1.1,,,,",
		// no "-" in plmn → skipped
		"XXX,UTC,N,xxx,500,600,5000,6000,9,999,oam3,core3,1.1.1.1,,,,,,",
	}
	return strings.Join(rows, "\n")
}

func TestParseCoreNRCSV(t *testing.T) {
	recs, err := ParseCoreNRCSV(strings.NewReader(sampleCoreCSV()))
	require.NoError(t, err)
	require.Len(t, recs, 2, "row with invalid plmn must be skipped")

	nyc := recs[0]
	assert.Equal(t, "NYC", nyc.Market)
	assert.Equal(t, "EST", nyc.Timezone)
	assert.Equal(t, "N", nyc.Dst)
	assert.Equal(t, "nyc", nyc.MarketAbbr)
	assert.Equal(t, 100, nyc.TacStart)
	assert.Equal(t, 200, nyc.TacEnd)
	assert.Equal(t, 1000, nyc.GnbIDStart)
	assert.Equal(t, 2000, nyc.GnbIDEnd)
	assert.Equal(t, "1|2|3", nyc.Pci)
	assert.Equal(t, "310", nyc.MCC)
	assert.Equal(t, "260", nyc.MNC)
	assert.Equal(t, "oam.example.com", nyc.OAMTunnelFQDN)
	assert.Equal(t, "core.example.com", nyc.CoreTunnelFQDN)
	assert.Equal(t, "10.0.0.1", nyc.AMFPoolDefaultIPv4)
	assert.Equal(t, "10.0.1.1;10.0.1.2", nyc.AMFPoolPrimaryIPv4)
	// MarketKey = Market_Timezone_Dst
	assert.Equal(t, "NYC_EST_N", nyc.MarketKey)

	lax := recs[1]
	assert.Equal(t, "311", lax.MCC)
	assert.Equal(t, "480", lax.MNC)
}

func TestParseCoreNRCSVNoData(t *testing.T) {
	_, err := ParseCoreNRCSV(strings.NewReader("market,plmn\n"))
	assert.Error(t, err, "header-only csv must error")
}

// ---------------------------------------------------------------------------
// N41_NR_SC KML parsing
// ---------------------------------------------------------------------------

func sampleKML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<kml xmlns="http://www.opengis.net/kml/2.2">
  <Document>
    <Folder>
      <Placemark>
        <name>NYC-1</name>
        <ExtendedData>
          <SchemaData>
            <SimpleData name="Market">NYC</SimpleData>
            <SimpleData name="RecordID">R1</SimpleData>
            <SimpleData name="CNTY_Name">New York</SimpleData>
            <SimpleData name="FreqBandIndicator">41</SimpleData>
            <SimpleData name="NRARFCN">504990</SimpleData>
            <SimpleData name="Bandwidth">100</SimpleData>
            <SimpleData name="RbNumber">273</SimpleData>
            <SimpleData name="AbsoluteFrequencySSB">504290</SimpleData>
          </SchemaData>
        </ExtendedData>
        <Polygon>
          <outerBoundaryIs>
            <LinearRing>
              <coordinates>
                -74.0,40.7,0 -74.0,40.8,0 -73.9,40.8,0 -73.9,40.7,0 -74.0,40.7,0
              </coordinates>
            </LinearRing>
          </outerBoundaryIs>
        </Polygon>
      </Placemark>
    </Folder>
  </Document>
</kml>`
}

func TestParseKML(t *testing.T) {
	list, err := ParseKML(strings.NewReader(sampleKML()))
	require.NoError(t, err)
	require.Len(t, list, 1)

	d := list[0]
	assert.Equal(t, "NYC", d.Market)
	assert.Equal(t, "R1", d.RecordID)
	assert.Equal(t, "New York", d.CountyName)
	assert.Equal(t, 504990, d.NRARFCN)
	assert.Equal(t, 100, d.Bandwidth)
	assert.Equal(t, 273, d.RbNumber)
	assert.Equal(t, 504290, d.SSB)
	require.Len(t, d.Polygon, 5, "KML closes the ring with a repeat first vertex")
	// x=longitude, y=latitude
	assert.Equal(t, Point{Lng: -74.0, Lat: 40.7}, d.Polygon[0])
	assert.Equal(t, Point{Lng: -73.9, Lat: 40.8}, d.Polygon[2])
}

// ---------------------------------------------------------------------------
// Geometry + selection
// ---------------------------------------------------------------------------

func square(lng0, lat0, lng1, lat1 float64) []Point {
	return []Point{
		{Lng: lng0, Lat: lat0},
		{Lng: lng0, Lat: lat1},
		{Lng: lng1, Lat: lat1},
		{Lng: lng1, Lat: lat0},
		{Lng: lng0, Lat: lat0},
	}
}

func TestPointInPolygon(t *testing.T) {
	poly := square(-74.0, 40.7, -73.9, 40.8)
	assert.True(t, pointInPolygon(poly, -73.95, 40.75), "centre inside")
	assert.False(t, pointInPolygon(poly, -80.0, 40.75), "far west outside")
	assert.False(t, pointInPolygon(poly, -73.95, 50.0), "far north outside")
	assert.False(t, pointInPolygon([]Point{{0, 0}, {1, 1}}, 0.5, 0.5), "degenerate polygon (<3) returns false")
}

func TestSelectSpatialFile(t *testing.T) {
	a := &SpatialFileDTO{Market: "A", Polygon: square(-74.0, 40.7, -73.9, 40.8)}
	b := &SpatialFileDTO{Market: "B", Polygon: square(-73.9, 40.7, -73.8, 40.8)}
	list := []*SpatialFileDTO{a, b}

	got := selectSpatialFile(-73.95, 40.75, list)
	require.NotNil(t, got)
	assert.Equal(t, "A", got.Market)

	got = selectSpatialFile(-73.85, 40.75, list)
	require.NotNil(t, got)
	assert.Equal(t, "B", got.Market)

	assert.Nil(t, selectSpatialFile(0, 0, list), "ocean point → no polygon")
	assert.Nil(t, selectSpatialFile(0, 0, nil), "nil list → nil")
}

func TestPickPCI(t *testing.T) {
	assert.Equal(t, 1, pickPCI("1|2|3"))
	assert.Equal(t, 42, pickPCI(" 42 | 7 "))
	assert.Equal(t, 0, pickPCI(""))
	assert.Equal(t, 0, pickPCI("abc|def"))
}

// ---------------------------------------------------------------------------
// resolveCoreValues (wired into processElement)
// ---------------------------------------------------------------------------

func TestResolveCoreValues(t *testing.T) {
	records := []*CoreRecord{
		{Market: "NYC", MarketKey: "NYC_EST_N", MCC: "310", MNC: "260", Pci: "1|2|3"},
		{Market: "LAX", MarketKey: "LAX_PST_Y", MCC: "311", MNC: "480", Pci: "4|5"},
	}
	nycPoly := &SpatialFileDTO{Market: "NYC", NRARFCN: 504990, Polygon: square(-74.0, 40.7, -73.9, 40.8)}
	laxPoly := &SpatialFileDTO{Market: "LAX", NRARFCN: 503790, Polygon: square(-118.3, 33.9, -118.2, 34.0)}
	spatial := []*SpatialFileDTO{nycPoly, laxPoly}

	t.Run("no core/spatial → defaults", func(t *testing.T) {
		em, mcc, mnc, pci, dl, ul, sh, ch := resolveCoreValues(0, 0, "X", nil, nil)
		assert.Equal(t, "X", em)
		assert.Equal(t, defaultMCC, mcc)
		assert.Equal(t, defaultMNC, mnc)
		assert.Equal(t, 0, pci)
		assert.Equal(t, 0, dl)
		assert.Equal(t, 0, ul)
		assert.False(t, sh)
		assert.False(t, ch)
	})

	t.Run("polygon selects market + arfcn, core supplies plmn/pci", func(t *testing.T) {
		em, mcc, mnc, pci, dl, ul, sh, ch := resolveCoreValues(40.75, -73.95, "deviceMarket", records, spatial)
		assert.Equal(t, "NYC", em)
		assert.Equal(t, "310", mcc)
		assert.Equal(t, "260", mnc)
		assert.Equal(t, 1, pci)
		assert.Equal(t, 504990, dl)
		assert.Equal(t, 504990, ul)
		assert.True(t, sh)
		assert.True(t, ch)
	})

	t.Run("polygon miss → device market fallback, core by MarketKey", func(t *testing.T) {
		// Point is inside laxPoly but the device market is LAX; the polygon
		// is empty here so effective market falls back to the device market.
		em, mcc, mnc, pci, dl, ul, sh, ch := resolveCoreValues(0, 0, "LAX_PST_Y", records, nil)
		assert.Equal(t, "LAX_PST_Y", em)
		assert.Equal(t, "311", mcc)
		assert.Equal(t, "480", mnc)
		assert.Equal(t, 4, pci)
		assert.Equal(t, 0, dl)
		assert.Equal(t, 0, ul)
		assert.False(t, sh)
		assert.True(t, ch)
	})

	t.Run("polygon hit but no matching core record → default plmn, arfcn still set", func(t *testing.T) {
		// LAX point, but no core record for LAX (only NYC exists).
		recordsOnlyNYC := []*CoreRecord{records[0]}
		em, mcc, mnc, pci, dl, ul, sh, ch := resolveCoreValues(33.95, -118.25, "LAX", recordsOnlyNYC, spatial)
		assert.Equal(t, "LAX", em) // from polygon
		assert.Equal(t, defaultMCC, mcc)
		assert.Equal(t, defaultMNC, mnc)
		assert.Equal(t, 503790, dl) // arfcn still resolved from polygon
		assert.Equal(t, 503790, ul)
		assert.Equal(t, 0, pci)
		assert.True(t, sh)
		assert.False(t, ch)
	})
}

// ---------------------------------------------------------------------------
// latestFile discovery
// ---------------------------------------------------------------------------

func TestLatestFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/CORE_NR_FEMTO_1.csv", []byte("x"), 0644))
	require.NoError(t, os.WriteFile(dir+"/CORE_NR_FEMTO_5.csv", []byte("x"), 0644))
	require.NoError(t, os.WriteFile(dir+"/CORE_NR_FEMTO_12.csv", []byte("x"), 0644))
	require.NoError(t, os.WriteFile(dir+"/CORE_NR_FEMTO_3.csv", []byte("x"), 0644))

	got, err := latestFile(dir, "CORE_NR_FEMTO_", ".csv")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "CORE_NR_FEMTO_12.csv"), got, "largest trailing number wins")

	none, err := latestFile(dir, "NOPE_", ".csv")
	require.NoError(t, err)
	assert.Equal(t, "", none, "no match → empty")
}

func TestLoadCoreData(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/CORE_NR_FEMTO_1.csv", []byte(sampleCoreCSV()), 0644))
	require.NoError(t, os.WriteFile(dir+"/N41_NR_SC_1.kml", []byte(sampleKML()), 0644))

	recs, spatial, err := LoadCoreData(dir)
	require.NoError(t, err)
	assert.Len(t, recs, 2, "invalid-plmn row skipped")
	assert.Len(t, spatial, 1)
	assert.Equal(t, "NYC", spatial[0].Market)

	// Missing directory → non-error, empty slices (Phase 2a fallback path).
	emptyRecs, emptySpatial, err := LoadCoreData(dir + "/does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, emptyRecs)
	assert.Empty(t, emptySpatial)
}
