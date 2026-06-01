//go:build windows

package sysctl

import "time"

type SystemCollector struct{}

func NewCollector() *SystemCollector                         { return &SystemCollector{} }
func (c *SystemCollector) Start(interval time.Duration)      {}
func (c *SystemCollector) Stop()                             {}
func (c *SystemCollector) Get() *SystemSample                { return &SystemSample{} }
