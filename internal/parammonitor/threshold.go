package parammonitor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"nmsappsrv/internal/alarm"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"

	goredis "github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// ThresholdRule model
// ---------------------------------------------------------------------------

// ThresholdRule defines a parameter monitoring threshold alert rule.
type ThresholdRule struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	ParameterName  string    `gorm:"column:parameter_name;type:varchar(255);not null" json:"parameter_name"`
	DeviceGroupID  uint      `gorm:"column:device_group_id" json:"device_group_id"`
	Operator       string    `gorm:"column:operator;type:varchar(10);not null" json:"operator"` // gt, lt, eq, ne, gte, lte
	ThresholdValue float64   `gorm:"column:threshold_value;not null" json:"threshold_value"`
	Severity       string    `gorm:"column:severity;type:varchar(20);not null" json:"severity"` // critical, major, minor, warning
	Enabled        bool      `gorm:"column:enabled;default:true" json:"enabled"`
	Description    string    `gorm:"column:description;type:varchar(500)" json:"description"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ThresholdRule) TableName() string { return "threshold_rule" }

// ---------------------------------------------------------------------------
// ThresholdViolation
// ---------------------------------------------------------------------------

// ThresholdViolation represents a single threshold breach for a device.
type ThresholdViolation struct {
	RuleID         uint    `json:"rule_id"`
	DeviceSN       string  `json:"device_sn"`
	ParameterName  string  `json:"parameter_name"`
	ActualValue    float64 `json:"actual_value"`
	ThresholdValue float64 `json:"threshold_value"`
	Severity       string  `json:"severity"`
}

// ---------------------------------------------------------------------------
// ThresholdChecker
// ---------------------------------------------------------------------------

// ThresholdChecker periodically evaluates threshold rules against live
// parameter values and creates alarms for violations.
type ThresholdChecker struct {
	db       *gorm.DB
	rdb      *goredis.Client
	alarmSvc alarm.Service
	stopCh   chan struct{}
}

// NewThresholdChecker creates a ThresholdChecker.
func NewThresholdChecker(db *gorm.DB, rdb *goredis.Client, alarmSvc alarm.Service) *ThresholdChecker {
	return &ThresholdChecker{
		db:       db,
		rdb:      rdb,
		alarmSvc: alarmSvc,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic threshold checking loop (every 60 seconds).
func (tc *ThresholdChecker) Start() {
	utils.SafeGo("threshold-checker", func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := tc.checkThresholds(); err != nil {
					logger.Errorf("threshold check error: %v", err)
				}
			case <-tc.stopCh:
				logger.Info("threshold checker stopped")
				return
			}
		}
	})
}

// Stop signals the background checker goroutine to exit.
func (tc *ThresholdChecker) Stop() {
	close(tc.stopCh)
}

// checkThresholds loads all enabled rules and evaluates each one.
// If a single rule fails, the error is logged and the remaining rules continue.
func (tc *ThresholdChecker) checkThresholds() error {
	var rules []ThresholdRule
	if err := tc.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return fmt.Errorf("load threshold rules: %w", err)
	}

	for i := range rules {
		violations, err := tc.evaluateRule(&rules[i])
		if err != nil {
			logger.Errorf("evaluate rule %d (%s) error: %v", rules[i].ID, rules[i].Name, err)
			continue
		}
		for _, v := range violations {
			if err := tc.createAlarmFromViolation(v); err != nil {
				logger.Errorf("create alarm for violation rule=%d device=%s: %v", v.RuleID, v.DeviceSN, err)
			}
		}
	}
	return nil
}

// ParameterRecord holds the latest parameter reading for a device.
type ParameterRecord struct {
	ElementID  int64
	DeviceSN   string
	ParamName  string
	ParamValue string
}

// evaluateRule checks a single rule against all current parameter values.
func (tc *ThresholdChecker) evaluateRule(rule *ThresholdRule) ([]ThresholdViolation, error) {
	var records []ParameterRecord

	query := tc.db.Table("element_basic_info_parameter").
		Select("element_basic_info_parameter.element_id, cpe_element.serial_number, element_basic_info_parameter.param_name, element_basic_info_parameter.param_value").
		Joins("JOIN cpe_element ON cpe_element.ne_neid = element_basic_info_parameter.element_id AND cpe_element.deleted = 0").
		Where("element_basic_info_parameter.param_name = ?", rule.ParameterName)

	// Optionally scope to a device group.
	if rule.DeviceGroupID > 0 {
		query = query.Where("element_basic_info_parameter.element_id IN (?)",
			tc.db.Table("group_has_element").
				Select("element_id").
				Where("group_id = ?", rule.DeviceGroupID),
		)
	}

	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query parameter values: %w", err)
	}

	var violations []ThresholdViolation
	for _, rec := range records {
		val, err := strconv.ParseFloat(rec.ParamValue, 64)
		if err != nil {
			continue // non-numeric value, skip silently
		}
		if compareOperator(rule.Operator, val, rule.ThresholdValue) {
			violations = append(violations, ThresholdViolation{
				RuleID:         rule.ID,
				DeviceSN:       rec.DeviceSN,
				ParameterName:  rule.ParameterName,
				ActualValue:    val,
				ThresholdValue: rule.ThresholdValue,
				Severity:       rule.Severity,
			})
		}
	}
	return violations, nil
}

// createAlarmFromViolation creates an alarm via alarm.Service with Redis-based
// deduplication (10-minute TTL) to prevent alarm storms.
func (tc *ThresholdChecker) createAlarmFromViolation(v ThresholdViolation) error {
	// Deduplication: skip if an alarm was recently created for the same rule+device.
	ctx := context.Background()
	dedupKey := fmt.Sprintf("param:threshold:alarm:%d:%s", v.RuleID, v.DeviceSN)
	if tc.rdb.Exists(ctx, dedupKey).Val() > 0 {
		return nil
	}

	now := time.Now()
	alarmIdentifier := fmt.Sprintf("ParameterThreshold_%d", v.RuleID)
	probableCause := fmt.Sprintf("Parameter %s threshold breached", v.ParameterName)
	additionalInfo := fmt.Sprintf("Actual: %.2f, Threshold: %.2f, Operator breached on rule %d",
		v.ActualValue, v.ThresholdValue, v.RuleID)

	a := &alarm.Alarm{
		Severity:        &v.Severity,
		AlarmIdentifier: &alarmIdentifier,
		ProbableCause:   &probableCause,
		AlarmSource:     &v.ParameterName,
		EventType:       strPtr("threshold_violation"),
		AlarmStatus:     intPtr(1),
		AlarmType:       intPtr(4), // communications alarm
		EventTime:       &now,
		SpecificProblem: &additionalInfo,
		CreateTime:      &now,
		UpdateTime:      &now,
	}

	if err := tc.alarmSvc.CreateAlarm(a); err != nil {
		return fmt.Errorf("create alarm: %w", err)
	}

	// Set dedup key with 10-minute TTL.
	tc.rdb.Set(ctx, dedupKey, "1", 10*time.Minute)
	return nil
}

// compareOperator evaluates whether value satisfies the threshold condition.
func compareOperator(op string, value, threshold float64) bool {
	switch op {
	case "gt":
		return value > threshold
	case "lt":
		return value < threshold
	case "eq":
		return value == threshold
	case "ne":
		return value != threshold
	case "gte":
		return value >= threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}

// --- small helpers ---

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
