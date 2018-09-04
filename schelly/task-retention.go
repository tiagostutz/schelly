package main

import (
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
)

var runningTask = false

func runRetentionTask() {
	if runningTask {
		logrus.Debug("runRetentionTask already running. skipping new task creation")
		return
	} else {
		runningTask = true
	}
	triggerRetentionTask()
	runningTask = false
}

func triggerRetentionTask() {
	start := time.Now()
	logrus.Info("")
	logrus.Info(">>>> BACKUP RETENTION MANAGEMENT")

	// backups, err := getAllMaterializedBackups(0)
	// if err != nil {
	// 	logrus.Errorf("Couldn't get materialized_backups. err=%s", err)
	// 	return
	// }
	// logrus.Debugf("Found %d backup references in local database", len(backups))

	logrus.Debugf("Retention policy: minutely=%s, hourly=%s, daily=%s, weekly=%s, monthly=%s, yearly=%s", options.minutelyParams[0], options.hourlyParams[0], options.dailyParams[0], options.weeklyParams[0], options.monthlyParams[0], options.yearlyParams[0])

	electedBackups := make([]MaterializedBackup, 0)
	electedBackups = appendElectedForTag("", "0", electedBackups)
	electedBackups = appendElectedForTag("minutely", options.minutelyParams[0], electedBackups)
	electedBackups = appendElectedForTag("hourly", options.hourlyParams[0], electedBackups)
	electedBackups = appendElectedForTag("daily", options.dailyParams[0], electedBackups)
	electedBackups = appendElectedForTag("weekly", options.weeklyParams[0], electedBackups)
	electedBackups = appendElectedForTag("monthly", options.monthlyParams[0], electedBackups)
	electedBackups = appendElectedForTag("yearly", options.yearlyParams[0], electedBackups)
	logrus.Infof("%d backups elected for deletion", len(electedBackups))

	for _, backup := range electedBackups {
		logrus.Debugf("Deleting backup '%s'...", backup.ID)
		res, err := setStatusMaterializedBackup(backup.ID, "deleting")
		ra, _ := res.RowsAffected()
		if err != nil {
			logrus.Errorf("Couldn't set status of backup '%s' to 'deleting'. Skipping backup deletion. err=%s", backup.ID, err)
		} else if ra != 1 {
			logrus.Errorf("Strange number of affected rows while setting status of backup '%s' to 'deleting'. Skipping backup deletion. rowsAffected=%d", backup.ID, ra)
		} else {
			// res, err2 := getMaterializedBackup(backup.ID)
			// if err2 != nil {
			// 	logrus.Errorf(">>>>>ERR %s", err2)
			// } else {
			// 	logrus.Infof(">>>>>INFO %s", res)
			// }
			// logrus.Debugf(">>>>>>>CHANGED TO DELETING")
			err = deleteWebhookBackup(backup.ID)
			if err != nil {
				logrus.Warnf("Could not delete backup '%s' using webhook. err=%s", backup.ID, err)
				res, err = setStatusMaterializedBackup(backup.ID, "delete-error")
				if err != nil {
					logrus.Warnf("Could not set backup %s status to 'delete-error'. err=%s", backup.ID, err)
				}
			} else {
				logrus.Infof("Backup '%s' deleted successfuly", backup.ID)
				res, err = setStatusMaterializedBackup(backup.ID, "deleted")
				if err != nil {
					logrus.Warnf("Could not set backup %s status to 'deleted'. err=%s", backup.ID, err)
				}
			}
		}
	}

	elapsed := time.Now().Sub(start)
	logrus.Infof("Retention management task done. elapsed=%s", elapsed)
}

func retryDeleteErrors() {
	logrus.Debugf("Retrying webhook delete for backups with 'delete-error' tag")
	backups, err := getMaterializedBackups(10, "delete-error")
	if err != nil {
		logrus.Errorf("Couldn't query backups tagged as 'delete-error'. err=%s", err)
	} else if len(backups) > 0 {
		logrus.Infof("%d backups tagged with 'backup-error'. retrying to delete them on webhook (limited to 10 at each retry)", len(backups))
		for _, backup := range backups {
			err = deleteWebhookBackup(backup.ID)
			if err != nil {
				logrus.Warnf("Could not delete backup '%s' using webhook. err=%s", backup.ID, err)
			}
		}
	} else {
		logrus.Debugf("No backups tagged with 'delete-error'")
	}
}

func appendElectedForTag(tag string, retentionCount string, appendTo []MaterializedBackup) []MaterializedBackup {
	ret, err0 := strconv.Atoi(retentionCount)
	if err0 != nil {
		logrus.Errorf("%s: Invalid retention parameter: err=%s", tag, err0)
		return appendTo
	}
	mbackups, err := getExclusiveTagAvailableMaterializedBackups(tag, ret, 10)
	if err != nil {
		logrus.Errorf("%s: Error querying backups for deletion. err=%s", tag, err)
		return appendTo
	}
	logrus.Debugf("%s: %d backups elected for deletion (limited to 10)", tag, len(mbackups))
	return append(appendTo, mbackups...)
}