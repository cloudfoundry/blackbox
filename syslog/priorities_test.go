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
	"testing"
)

func TestLookupSeverity(t *testing.T) {
	var sev Priority
	var err error

	sev, err = Severity("warn")
	if sev != SevWarning && err != nil {
		t.Errorf("Failed to lookup severity warning")
	}

	sev, err = Severity("foo")
	if sev != 0 && err != ErrPriority {
		t.Errorf("Failed to lookup severity foo")
	}

	sev, err = Severity("")
	if sev != 0 && err != ErrPriority {
		t.Errorf("Failed to lookup empty severity")
	}
}

func TestLookupFacility(t *testing.T) {
	var facility Priority
	var err error

	facility, err = Facility("local1")
	if facility != LogLocal1 && err != nil {
		t.Errorf("Failed to lookup facility local1")
	}

	facility, err = Facility("foo")
	if facility != 0 && err != ErrPriority {
		t.Errorf("Failed to lookup facility foo")
	}

	facility, err = Facility("")
	if facility != 0 && err != ErrPriority {
		t.Errorf("Failed to lookup empty facility")
	}
}
