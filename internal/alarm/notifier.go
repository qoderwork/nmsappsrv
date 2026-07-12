package alarm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/mail"
	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"
	redisclient "nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// AlarmNotifier checks whether an alarm matches a template with email
// notifications enabled and sends the email accordingly.
type AlarmNotifier struct {
	db          *gorm.DB
	mailService mail.Service

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewAlarmNotifier creates an AlarmNotifier.
func NewAlarmNotifier(db *gorm.DB, mailService mail.Service) *AlarmNotifier {
	return &AlarmNotifier{
		db:          db,
		mailService: mailService,
		stopCh:      make(chan struct{}),
	}
}

// Start subscribes to the alarm-notify pub/sub channel and processes messages.
func (n *AlarmNotifier) Start() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.running {
		return
	}
	n.running = true
	n.stopCh = make(chan struct{})

	utils.SafeGo("alarm-notifier", func() {
		n.run()
	})

	logger.Infof("alarm notifier started")
}

// Stop signals the background goroutine to exit.
func (n *AlarmNotifier) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.running {
		return
	}
	n.running = false
	close(n.stopCh)
	logger.Infof("alarm notifier stopped")
}

// IsRunning returns whether the notifier background worker is active.
func (n *AlarmNotifier) IsRunning() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.running
}

// run is the main loop that listens for alarm notifications on Redis pub/sub.
func (n *AlarmNotifier) run() {
	ctx := context.Background()
	pubsub := redisclient.Subscribe(ctx, mq.ChannelAlarmNotify)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-n.stopCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			n.handleMessage(ctx, msg.Payload)
		}
	}
}

// handleMessage parses the alarm ID from the pub/sub message and triggers
// email notification.
func (n *AlarmNotifier) handleMessage(_ context.Context, payload string) {
	alarmID, err := strconv.ParseInt(strings.TrimSpace(payload), 10, 64)
	if err != nil {
		logger.Errorf("alarm notifier: invalid alarm id %q: %v", payload, err)
		return
	}

	var a Alarm
	if err := n.db.Where("id = ?", alarmID).First(&a).Error; err != nil {
		logger.Errorf("alarm notifier: failed to find alarm %d: %v", alarmID, err)
		return
	}

	n.NotifyAlarm(&a)
}

// NotifyAlarmAsync wraps NotifyAlarm in a SafeGo call so the caller does not
// block while the email is being composed and sent.
func (n *AlarmNotifier) NotifyAlarmAsync(a *Alarm) {
	alarmID := a.Id
	utils.SafeGo(fmt.Sprintf("alarm-notify-%d", alarmID), func() {
		var fresh Alarm
		if err := n.db.Where("id = ?", alarmID).First(&fresh).Error; err != nil {
			logger.Errorf("alarm notifier async: failed to load alarm %d: %v", alarmID, err)
			return
		}
		n.NotifyAlarm(&fresh)
	})
}

// NotifyAlarm is the core logic: find the matching template, check interval,
// build the email, and send it.
func (n *AlarmNotifier) NotifyAlarm(a *Alarm) {
	// 1. Find the matching AlarmTemplate.
	tmpl, err := n.findTemplate(a)
	if err != nil {
		logger.Errorf("alarm notifier: find template for alarm %d: %v", a.Id, err)
		return
	}
	if tmpl == nil {
		logger.Debugf("alarm notifier: no matching template for alarm %d", a.Id)
		return
	}

	// 2. Check if email notification is enabled.
	if tmpl.EnableEmailNotification == nil || !*tmpl.EnableEmailNotification {
		return
	}

	// 3. Check interval - has enough time passed since the last email?
	if !n.isIntervalElapsed(tmpl) {
		return
	}

	// 4. Build recipient list.
	recipients := n.buildRecipientList(tmpl)
	if len(recipients) == 0 {
		logger.Debugf("alarm notifier: no recipients for template %d", tmpl.Id)
		return
	}

	// 5. Build email content.
	subject := n.buildSubject(a)
	htmlBody := n.buildHTMLBody(a)

	// 6. Send the email.
	if err := n.mailService.SendMail(recipients, subject, htmlBody); err != nil {
		logger.Errorf("alarm notifier: failed to send email for alarm %d: %v", a.Id, err)
		return
	}
	logger.Infof("alarm notifier: sent email for alarm %d to %v", a.Id, recipients)

	// 7. Update LastSendEmailDate on the template.
	now := time.Now()
	n.db.Model(&AlarmTemplate{}).Where("id = ?", tmpl.Id).
		Update("last_send_email_date", now)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// findTemplate locates the AlarmTemplate that matches the given alarm.
// It first tries the explicit alarm_template_id, then falls back to matching
// by alarm_identifier against templates whose alarm_ids contain it.
func (n *AlarmNotifier) findTemplate(a *Alarm) (*AlarmTemplate, error) {
	// Direct reference via alarm_template_id.
	if a.AlarmTemplateId != nil && *a.AlarmTemplateId > 0 {
		var tmpl AlarmTemplate
		if err := n.db.Where("id = ?", *a.AlarmTemplateId).First(&tmpl).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			return nil, err
		}
		return &tmpl, nil
	}

	// Fallback: look for templates whose alarm_ids field contains the alarm identifier.
	if a.AlarmIdentifier != nil && *a.AlarmIdentifier != "" {
		var tmpl AlarmTemplate
		err := n.db.Where("alarm_ids LIKE ?", "%"+*a.AlarmIdentifier+"%").First(&tmpl).Error
		if err == nil {
			return &tmpl, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	return nil, nil
}

// isIntervalElapsed checks whether enough time has passed since the last email
// was sent for this template, based on the template Interval_ (minutes).
func (n *AlarmNotifier) isIntervalElapsed(tmpl *AlarmTemplate) bool {
	if tmpl.LastSendEmailDate == nil {
		return true // never sent before
	}
	intervalMinutes := 30 // default
	if tmpl.Interval_ != nil && *tmpl.Interval_ > 0 {
		intervalMinutes = *tmpl.Interval_
	}
	elapsed := time.Since(*tmpl.LastSendEmailDate)
	return elapsed >= time.Duration(intervalMinutes)*time.Minute
}

// buildRecipientList collects email addresses from the template Emails field
// and, when EnableNotifyDefaultRecipients is set, the super-user emails from
// the mail configuration.
func (n *AlarmNotifier) buildRecipientList(tmpl *AlarmTemplate) []string {
	seen := make(map[string]bool)
	var recipients []string

	addAddr := func(raw string) {
		for _, addr := range strings.Split(raw, ";") {
			addr = strings.TrimSpace(addr)
			if addr != "" && !seen[addr] {
				seen[addr] = true
				recipients = append(recipients, addr)
			}
		}
	}

	// Template-level emails.
	if tmpl.Emails != nil {
		addAddr(*tmpl.Emails)
	}

	// Default recipients (super-user).
	if tmpl.EnableNotifyDefaultRecipients != nil && *tmpl.EnableNotifyDefaultRecipients {
		if superEmail, err := n.mailService.GetSuperUserEmail(); err == nil {
			addAddr(superEmail)
		} else {
			logger.Errorf("alarm notifier: failed to get super user email: %v", err)
		}
	}

	return recipients
}

// buildSubject creates the email subject line.
func (n *AlarmNotifier) buildSubject(a *Alarm) string {
	severity := derefOr(a.Severity, "Unknown")
	source := derefOr(a.AlarmSource, "Unknown")
	return fmt.Sprintf("[NMS Alarm] %s - %s", severity, source)
}

// buildHTMLBody creates an HTML email body with alarm details.
func (n *AlarmNotifier) buildHTMLBody(a *Alarm) string {
	severity := derefOr(a.Severity, "N/A")
	source := derefOr(a.AlarmSource, "N/A")
	identifier := derefOr(a.AlarmIdentifier, "N/A")
	networkElement := derefOr(a.NetworkElement, "N/A")
	probableCause := derefOr(a.ProbableCause, "N/A")
	specificProblem := derefOr(a.SpecificProblem, "N/A")
	additionalInfo := derefOr(a.AdditionalInformation, "N/A")
	eventTime := "N/A"
	if a.EventTime != nil {
		eventTime = a.EventTime.Format("2006-01-02 15:04:05")
	}

	return fmt.Sprintf(
		"<html><body style=\"font-family:Arial,sans-serif;\">" +
			"<h2 style=\"color:#d9534f;\">NMS Alarm Notification</h2>" +
			"<table border=\"1\" cellpadding=\"8\" cellspacing=\"0\" style=\"border-collapse:collapse;\">" +
			"<tr><td><b>Severity</b></td><td>%s</td></tr>" +
			"<tr><td><b>Alarm Source</b></td><td>%s</td></tr>" +
			"<tr><td><b>Alarm Identifier</b></td><td>%s</td></tr>" +
			"<tr><td><b>Network Element</b></td><td>%s</td></tr>" +
			"<tr><td><b>Probable Cause</b></td><td>%s</td></tr>" +
			"<tr><td><b>Specific Problem</b></td><td>%s</td></tr>" +
			"<tr><td><b>Event Time</b></td><td>%s</td></tr>" +
			"<tr><td><b>Additional Info</b></td><td>%s</td></tr>" +
			"</table>" +
			"<p style=\"color:#888;font-size:12px;\">This is an automated message from NMS.</p>" +
			"</body></html>",
		severity, source, identifier, networkElement, probableCause, specificProblem, eventTime, additionalInfo)
}

// derefOr returns the dereferenced string or a fallback if the pointer is nil.
func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
