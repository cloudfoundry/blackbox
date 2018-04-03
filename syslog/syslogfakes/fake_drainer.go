// This file was generated by counterfeiter
package syslogfakes

import (
	"sync"

	"github.com/cloudfoundry/blackbox/syslog"
)

type FakeDrainer struct {
	DrainStub        func(line string, tag string) error
	drainMutex       sync.RWMutex
	drainArgsForCall []struct {
		line string
		tag  string
	}
	drainReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeDrainer) Drain(line string, tag string) error {
	fake.drainMutex.Lock()
	fake.drainArgsForCall = append(fake.drainArgsForCall, struct {
		line string
		tag  string
	}{line, tag})
	fake.recordInvocation("Drain", []interface{}{line, tag})
	fake.drainMutex.Unlock()
	if fake.DrainStub != nil {
		return fake.DrainStub(line, tag)
	} else {
		return fake.drainReturns.result1
	}
}

func (fake *FakeDrainer) DrainCallCount() int {
	fake.drainMutex.RLock()
	defer fake.drainMutex.RUnlock()
	return len(fake.drainArgsForCall)
}

func (fake *FakeDrainer) DrainArgsForCall(i int) (string, string) {
	fake.drainMutex.RLock()
	defer fake.drainMutex.RUnlock()
	return fake.drainArgsForCall[i].line, fake.drainArgsForCall[i].tag
}

func (fake *FakeDrainer) DrainReturns(result1 error) {
	fake.DrainStub = nil
	fake.drainReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeDrainer) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.drainMutex.RLock()
	defer fake.drainMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeDrainer) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ syslog.Drainer = new(FakeDrainer)