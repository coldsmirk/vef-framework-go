package config

import (
	"errors"
	"fmt"
)

var errInvalidWorkday = errors.New("vef.calendar.workdays: invalid ISO weekday")

// CalendarConfig configures the baseline and optional override store.
type CalendarConfig struct {
	Workdays []int               `config:"workdays"`
	Store    CalendarStoreConfig `config:"store"`
}

// CalendarStoreConfig enables persisted overrides on the primary data source.
type CalendarStoreConfig struct {
	Enabled     bool `config:"enabled"`
	AutoMigrate bool `config:"auto_migrate"`
}

// EffectiveWorkdays defaults an omitted baseline to Monday through Friday.
// An explicitly empty list makes every baseline date a rest day.
func (c *CalendarConfig) EffectiveWorkdays() []int {
	if c.Workdays == nil {
		return []int{1, 2, 3, 4, 5}
	}

	return c.Workdays
}

// Validate checks that the configured workdays use ISO weekday numbers.
func (c *CalendarConfig) Validate() error {
	for _, day := range c.Workdays {
		if day < 1 || day > 7 {
			return fmt.Errorf("%w %d", errInvalidWorkday, day)
		}
	}

	return nil
}
