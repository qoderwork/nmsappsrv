package restapi

import (
	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ============================
// Alarm operations
// ============================

func (s *Service) ListAlarms(c *gin.Context, offset, limit int) ([]RestAlarmVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	alarms, total, err := s.repo.ListAlarms(licenseId, offset, limit)
	if err != nil {
		return nil, 0, apperror.Wrap(err, "LIST_ALARMS_FAILED", 500, "failed to list alarms")
	}

	var result []RestAlarmVo
	for _, a := range alarms {
		vo := RestAlarmVo{
			Id:              a.Id,
			Severity:        derefStr(a.Severity),
			AlarmIdentifier: derefStr(a.AlarmIdentifier),
			ProbableCause:   derefStr(a.ProbableCause),
			AlarmStatus:     derefIntPtr(a.AlarmStatus),
			EventType:       derefStr(a.EventType),
			ElementId:       derefInt64Ptr(a.ElementId),
			EventTime:       formatTime(a.EventTime),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *Service) SyncAlarms(c *gin.Context, req *SyncAlarmRequest) ([]RestAlarmVo, error) {
	licenseId := middleware.GetLicenseId(c)

	alarms, err := s.repo.SyncAlarms(req.ElementIds, licenseId)
	if err != nil {
		return nil, apperror.Wrap(err, "SYNC_ALARMS_FAILED", 500, "failed to sync alarms")
	}

	var result []RestAlarmVo
	for _, a := range alarms {
		vo := RestAlarmVo{
			Id:              a.Id,
			Severity:        derefStr(a.Severity),
			AlarmIdentifier: derefStr(a.AlarmIdentifier),
			ProbableCause:   derefStr(a.ProbableCause),
			AlarmStatus:     derefIntPtr(a.AlarmStatus),
			EventType:       derefStr(a.EventType),
			ElementId:       derefInt64Ptr(a.ElementId),
			EventTime:       formatTime(a.EventTime),
		}
		result = append(result, vo)
	}

	return result, nil
}

func (s *Service) ClearAlarms(c *gin.Context, req *ClearAlarmRequest) error {
	licenseId := middleware.GetLicenseId(c)
	username := middleware.GetUsername(c)

	if err := s.repo.ClearAlarms(req.AlarmIds, licenseId); err != nil {
		logger.Errorf("Failed to clear alarms: %v", err)
		return apperror.Wrap(err, "CLEAR_ALARMS_FAILED", 500, "failed to clear alarms")
	}

	logger.Infof("Cleared %d alarms by user %s", len(req.AlarmIds), username)
	return nil
}
