/*
Copyright (c) 2013 Paul Morton, Papertrail, Inc., & Paul Hammond

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package syslog

import (
	"fmt"
)

// A Syslog Priority is a combination of Severity and Facility.
type Priority int

// Returned when looking up a non-existant facility or severity
var ErrPriority = fmt.Errorf("Not a designated priority")

// RFC5424 Severities
const (
	SevEmerg Priority = iota
	SevAlert
	SevCrit
	SevErr
	SevWarning
	SevNotice
	SevInfo
	SevDebug
)

var severities = map[string]Priority{
	"emerg":  SevEmerg,
	"alert":  SevAlert,
	"crit":   SevCrit,
	"err":    SevErr,
	"warn":   SevWarning,
	"notice": SevNotice,
	"info":   SevInfo,
	"debug":  SevDebug,
}

// Severity returns the named severity. It returns ErrPriority if the severity
// does not exist.
func Severity(name string) (Priority, error) {
	p, ok := severities[name]
	if !ok {
		return 0, ErrPriority
	}
	return p, nil
}

// RFC5424 Facilities
const (
	LogKern Priority = iota
	LogUser
	LogMail
	LogDaemon
	LogAuth
	LogSyslog
	LogLPR
	LogNews
	LogUUCP
	LogCron
	LogAuthPriv
	LogFTP
	LogNTP
	LogAudit
	LogAlert
	LogAt
	LogLocal0
	LogLocal1
	LogLocal2
	LogLocal3
	LogLocal4
	LogLocal5
	LogLocal6
	LogLocal7
)

var facilities = map[string]Priority{
	"kern":     LogKern,
	"user":     LogUser,
	"mail":     LogMail,
	"daemon":   LogDaemon,
	"auth":     LogAuth,
	"syslog":   LogSyslog,
	"lpr":      LogLPR,
	"news":     LogNews,
	"uucp":     LogUUCP,
	"cron":     LogCron,
	"authpriv": LogAuthPriv,
	"ftp":      LogFTP,
	"ntp":      LogNTP,
	"audit":    LogAudit,
	"alert":    LogAlert,
	"at":       LogAt,
	"local0":   LogLocal0,
	"local1":   LogLocal1,
	"local2":   LogLocal2,
	"local3":   LogLocal3,
	"local4":   LogLocal4,
	"local5":   LogLocal5,
	"local6":   LogLocal6,
	"local7":   LogLocal7,
}

// Facility returns the named facility. It returns ErrPriority if the facility
// does not exist.
func Facility(name string) (Priority, error) {
	p, ok := facilities[name]
	if !ok {
		return 0, ErrPriority
	}
	return p, nil
}
