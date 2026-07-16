package ztp

import (
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// CoreFileDir is the directory the ZTP core files are read from. Java uses
// /home/files/north-file/ztp/. Override per deployment if the Go service
// mounts the files elsewhere. When the directory is absent the orchestrator
// falls back to the default PLMN (310/260) and skips geofence polygon
// selection — matching Phase 2a behaviour.
var CoreFileDir = "/home/files/north-file/ztp/"

// Point is a longitude/latitude pair (mirrors Java Point2D.Double(lng, lat);
// the ray-casting test uses x=longitude, y=latitude).
type Point struct {
	Lng float64
	Lat float64
}

// CoreRecord is one parsed CORE_NR_FEMTO_*.csv row. It supplies the PLMN
// (MCC/MNC), TAC range, PCI pool and market abbreviation used during ZTP.
type CoreRecord struct {
	MarketKey  string
	Market     string
	Timezone   string
	Dst        string
	MarketAbbr string
	TacStart   int
	TacEnd     int
	GnbIDStart int
	GnbIDEnd   int
	Pci        string // pipe-separated; one is picked per device
	MCC        string
	MNC        string
	// AMF pool / tunnel FQDNs are carried for AOS file generation.
	AMFPoolDefaultIPv4   string
	AMFPoolDefaultIPv6   string
	AMFPoolPrimaryIPv4   string
	AMFPoolSecondaryIPv4 string
	AMFPoolPrimaryIPv6   string
	AMFPoolSecondaryIPv6 string
	OAMTunnelFQDN         string
	CoreTunnelFQDN        string
}

// SpatialFileDTO is one parsed N41_NR_SC_*.kml <Placemark>: a geofence market
// polygon plus its RF parameters (nRARFCN is used as the cell ARFCN DL/UL).
type SpatialFileDTO struct {
	Market     string
	RecordID   string
	CountyName string
	NRARFCN    int
	Bandwidth  int
	RbNumber   int
	SSB        int
	Polygon    []Point
}

// ---------------------------------------------------------------------------
// CORE_NR_FEMTO_*.csv
// ---------------------------------------------------------------------------

// ParseCoreNRCSV parses a CORE_NR_FEMTO CSV (US-ASCII / UTF-8) into records.
// The 18-column header is matched by (lowercased) name so column order is
// tolerated. Rows whose plmn lacks a "-" (no MCC-MNC split) are skipped.
func ParseCoreNRCSV(r io.Reader) ([]*CoreRecord, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read core csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("core csv has no data rows")
	}
	col := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(row []string, name string) string {
		if i, ok := col[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	atoi := func(s string) int {
		if s == "" {
			return 0
		}
		v, _ := strconv.Atoi(s)
		return v
	}
	var out []*CoreRecord
	for _, row := range rows[1:] {
		plmn := get(row, "plmn")
		mcc, mnc := "", ""
		if i := strings.Index(plmn, "-"); i >= 0 {
			mcc, mnc = plmn[:i], plmn[i+1:]
		}
		if mcc == "" || mnc == "" {
			continue // invalid plmn: skip (Java faults the row)
		}
		rec := &CoreRecord{
			Market:               get(row, "market"),
			Timezone:             get(row, "timezone"),
			Dst:                  get(row, "dst"),
			MarketAbbr:           get(row, "market_abbr"),
			TacStart:             atoi(get(row, "tac_start")),
			TacEnd:               atoi(get(row, "tac_end")),
			GnbIDStart:           atoi(get(row, "gnbid_start")),
			GnbIDEnd:             atoi(get(row, "gnbid_end")),
			Pci:                  get(row, "pci"),
			MCC:                  mcc,
			MNC:                  mnc,
			AMFPoolDefaultIPv4:   get(row, "amf_pool_default_ipv4_ip"),
			AMFPoolDefaultIPv6:   get(row, "amf_pool_default_ipv6_ip"),
			AMFPoolPrimaryIPv4:   get(row, "amf_pool_primary_ipv4_ips"),
			AMFPoolSecondaryIPv4: get(row, "amf_pool_secondary_ipv4_ips"),
			AMFPoolPrimaryIPv6:   get(row, "amf_pool_primary_ipv6_ips"),
			AMFPoolSecondaryIPv6: get(row, "amf_pool_secondary_ipv6_ips"),
			OAMTunnelFQDN:        get(row, "final_oam_tunnel_fqdn"),
			CoreTunnelFQDN:       get(row, "final_core_tunnel_fqdn"),
		}
		rec.MarketKey = rec.Market + "_" + rec.Timezone + "_" + rec.Dst
		out = append(out, rec)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// N41_NR_SC_*.kml
// ---------------------------------------------------------------------------

// ParseKML parses N41_NR_SC KML into SpatialFileDTOs using a streaming token
// walk, so it is tolerant of Folder/Document nesting depth. Each <Placemark>
// yields one record: <SimpleData name="..."> values populate the RF fields
// and <coordinates> (lng,lat[,alt] space-separated) populate the polygon.
func ParseKML(r io.Reader) ([]*SpatialFileDTO, error) {
	dec := xml.NewDecoder(r)
	var list []*SpatialFileDTO
	var cur *SpatialFileDTO
	var sdName string
	var inSD, inCoords bool
	var sdBuf, coordBuf strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode kml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "placemark":
				cur = &SpatialFileDTO{}
			case "simpledata":
				for _, a := range t.Attr {
					if strings.EqualFold(a.Name.Local, "name") {
						sdName = a.Value
					}
				}
				inSD = true
				sdBuf.Reset()
			case "coordinates":
				inCoords = true
				coordBuf.Reset()
			}
		case xml.CharData:
			if inSD {
				sdBuf.Write(t)
			}
			if inCoords {
				coordBuf.Write(t)
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "simpledata":
				applySimpleData(cur, sdName, strings.TrimSpace(sdBuf.String()))
				sdName = ""
				inSD = false
			case "coordinates":
				if cur != nil {
					cur.Polygon = parseCoordinates(coordBuf.String())
				}
				inCoords = false
			case "placemark":
				if cur != nil {
					list = append(list, cur)
					cur = nil
				}
			}
		}
	}
	return list, nil
}

func applySimpleData(d *SpatialFileDTO, name, val string) {
	if d == nil {
		return
	}
	switch strings.ToLower(name) {
	case "market":
		d.Market = val
	case "recordid":
		d.RecordID = val
	case "cnty_name":
		d.CountyName = val
	case "nrarfcn":
		d.NRARFCN, _ = strconv.Atoi(val)
	case "bandwidth":
		d.Bandwidth, _ = strconv.Atoi(val)
	case "rbnumber":
		d.RbNumber, _ = strconv.Atoi(val)
	case "absolutefrequencyssb":
		d.SSB, _ = strconv.Atoi(val)
	}
}

// parseCoordinates parses "lng,lat[,alt] lng,lat[,alt] ..." into points.
func parseCoordinates(s string) []Point {
	var pts []Point
	for _, tok := range strings.Fields(s) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		parts := strings.Split(tok, ",")
		if len(parts) < 2 {
			continue
		}
		lng, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil {
			continue
		}
		pts = append(pts, Point{Lng: lng, Lat: lat})
	}
	return pts
}

// ---------------------------------------------------------------------------
// Geometry + selection
// ---------------------------------------------------------------------------

// pointInPolygon reports whether (x,y) is inside the polygon (ray casting,
// even-odd rule). Mirrors Java PositionUtil.isInPolygon (x=longitude,
// y=latitude).
func pointInPolygon(poly []Point, x, y float64) bool {
	n := len(poly)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i].Lng, poly[i].Lat
		xj, yj := poly[j].Lng, poly[j].Lat
		if ((yi > y) != (yj > y)) && (x < (xj-xi)*(y-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// selectSpatialFile returns the first polygon containing the device point,
// mirroring Java getSptialFileDTO. Returns nil when no polygon contains it.
func selectSpatialFile(lng, lat float64, list []*SpatialFileDTO) *SpatialFileDTO {
	for _, s := range list {
		if pointInPolygon(s.Polygon, lng, lat) {
			return s
		}
	}
	return nil
}

// pickPCI returns the first pipe-separated PCI value as an int (Java picks one
// at random; we pick the first for determinism).
func pickPCI(pci string) int {
	for _, p := range strings.Split(pci, "|") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if v, err := strconv.Atoi(p); err == nil {
			return v
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// File discovery + loading
// ---------------------------------------------------------------------------

// latestFile returns the path of the file matching basePrefix*<number>suffix
// with the largest trailing number (Java picks the newest core file).
func latestFile(dir, basePrefix, suffix string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, basePrefix+"*"+suffix))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	re := regexp.MustCompile(regexp.QuoteMeta(basePrefix) + `(\d+)` + regexp.QuoteMeta(suffix) + `$`)
	best, bestN := "", -1
	for _, m := range matches {
		sm := re.FindStringSubmatch(filepath.Base(m))
		if sm == nil {
			continue
		}
		n, _ := strconv.Atoi(sm[1])
		if n > bestN {
			bestN, best = n, m
		}
	}
	return best, nil
}

// LoadCoreData reads the latest CORE_NR_FEMTO CSV and N41_NR_SC KML from dir.
// A missing directory is not an error — it returns empty slices so the
// orchestrator keeps using its default PLMN and skips polygon selection.
func LoadCoreData(dir string) ([]*CoreRecord, []*SpatialFileDTO, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var recs []*CoreRecord
	if f, err := latestFile(dir, "CORE_NR_FEMTO_", ".csv"); err == nil && f != "" {
		fp, oerr := os.Open(f)
		if oerr != nil {
			return nil, nil, oerr
		}
		recs, err = ParseCoreNRCSV(fp)
		fp.Close()
		if err != nil {
			return nil, nil, err
		}
	}
	var spatial []*SpatialFileDTO
	if f, err := latestFile(dir, "N41_NR_SC_", ".kml"); err == nil && f != "" {
		fp, oerr := os.Open(f)
		if oerr != nil {
			return nil, nil, oerr
		}
		spatial, err = ParseKML(fp)
		fp.Close()
		if err != nil {
			return nil, nil, err
		}
	}
	return recs, spatial, nil
}
