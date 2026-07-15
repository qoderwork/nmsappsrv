package mr

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func mustTime(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.000", s)
	if err != nil {
		panic(err)
	}
	return t
}

const sampleMRO = `<?xml version="1.0" encoding="UTF-8"?>
<bulkPmMrDataFile>
  <fileHeader startTime="2024-01-01T00:00:00.000" endTime="2024-01-01T01:00:00.000"/>
  <gNB>
    <measurement>
      <smr>MR.NRScArfcn MR.NRScPci MR.NRScSSRSRP MR.NRScSSRSRQ MR.NRScSSSINR</smr>
      <measValue id="cell-1" AMFUENGAPID="ue-1" TimeStamp="2024-01-01T00:00:30.000" EventType="MR.LTE.MRE">100 1 70 50 33</measValue>
      <measValue id="cell-1" AMFUENGAPID="ue-2" TimeStamp="2024-01-01T00:00:40.000" EventType="MR.LTE.MRE">101 2 80 60 40</measValue>
    </measurement>
    <measurement>
      <smr>MR.NRScArfcn MR.NRScPci MR.NRScSSRSRP</smr>
      <measValue id="cell-2" AMFUENGAPID="ue-3" TimeStamp="2024-01-01T00:00:50.000" EventType="MR.LTE.MRE">200 9 90 77 55</measValue>
    </measurement>
  </gNB>
</bulkPmMrDataFile>`

func TestParseMRO(t *testing.T) {
	rows, err := ParseMRO(strings.NewReader(sampleMRO))
	assert.NoError(t, err)
	assert.Len(t, rows, 3)

	// first row of first measurement
	assert.Equal(t, "cell-1", rows[0].CellID)
	assert.Equal(t, "ue-1", rows[0].UeID)
	assert.Equal(t, "MR.LTE.MRE", rows[0].EventType)
	assert.Equal(t, "100", rows[0].NRScArfcn)
	assert.Equal(t, "1", rows[0].NRScPci)
	assert.Equal(t, "70", rows[0].NRScSSRSRP)
	assert.Equal(t, "50", rows[0].NRScSSRSRQ)
	assert.Equal(t, "33", rows[0].NRScSSSINR)
	assert.NotNil(t, rows[0].StartTime)
	assert.NotNil(t, rows[0].EndTime)
	assert.NotNil(t, rows[0].EventTime)

	// third row belongs to cell-2 (second measurement), smr had only 3 names
	assert.Equal(t, "cell-2", rows[2].CellID)
	assert.Equal(t, "200", rows[2].NRScArfcn)
	assert.Equal(t, "90", rows[2].NRScSSRSRP)
	assert.Equal(t, "", rows[2].NRScSSRSRQ) // no 4th name in this smr
}

func TestAggregateCell(t *testing.T) {
	rows := []MRData{
		{CellID: "c1", NRScSSRSRP: "70"}, // cov93 (>=64) cov109 (>=58) cov110 (>=57); avg=-87
		{CellID: "c1", NRScSSRSRP: "80"}, // all three; avg=-77
		{CellID: "c1", NRScSSRSRP: "40"}, // none; avg unchanged
	}
	start, end := mustTime("2024-01-01T00:00:00.000"), mustTime("2024-01-01T01:00:00.000")
	vo := aggregateCell("c1", "dev", "sn", start, end, rows)
	assert.Equal(t, "c1", vo.CellID)
	assert.Equal(t, "dev", vo.DeviceName)
	assert.Equal(t, 3, vo.RSRPTotalNumberOfSamplingPoints)
	// cov93: 70,80 qualify (>=64) -> 2
	assert.Equal(t, 2, vo.NumberOfEffectiveCoveringSamplingPointsGraterThan93)
	assert.Equal(t, 2, vo.NumberOfEffectiveCoveringSamplingPointsGraterThan110)
	// avg over all n>0 values: 70->-87, 80->-77, 40->-117 => -93.6667
	assert.Equal(t, "-93.6667", vo.AverageRSRP)
	// coverage rate 93 = 100*2/3 = 66.6667
	assert.Equal(t, "66.6667", vo.CoverageRateGreaterThan93)
}
