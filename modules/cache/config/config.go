package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// OSGroup represents os group configuration
type OSGroup struct {
	CacheWays uint   `toml:"cacheways"`
	CPUSet    string `toml:"cpuset"`
}

// InfraGroup represents infra group configuration
type InfraGroup struct {
	CacheWays uint     `toml:"cacheways"`
	CPUSet    string   `toml:"cpuset"`
	Tasks     []string `toml:"tasks"`
}

// CachePool represents cache pool layout configuration
type CachePool struct {
	MaxAllowedShared uint `toml:"max_allowed_shared"`
	Guarantee        uint `toml:"guarantee"`
	Besteffort       uint `toml:"besteffort"`
	Shared           uint `toml:"shared"`
	Shrink           bool `toml:"shrink"`
}

// RDTConfig contains RDT related configuration flags from rmd.toml
type RDTConfig struct {
	MBAMode string `toml:"mbaMode"`
}

// MBAModeToInt converts suppored MBA modes (none, percentage or mbps) into PQOS compatible values (-1, 0 and 1 respectively)
//
// return error if invalid mode given
func MBAModeToInt(mode string) (int, error) {
	switch mode {
	case "none":
		return -1, nil
	case "percentage":
		return 0, nil
	case "mbps":
		return 1, nil
	default:
		return -2, fmt.Errorf("Unsupported MBA mode: [%v]", mode)
	}
}

var infraConfigOnce sync.Once
var osConfigOnce sync.Once
var cachePoolConfigOnce sync.Once

var infragroup = &InfraGroup{}
var osgroup = &OSGroup{1, "0"}

// FIXME: the default may not work on some platform
var cachepool = &CachePool{10, 10, 7, 2, false}

// NewInfraConfig reads InfraGroup configuration
func NewInfraConfig() *InfraGroup {
	infraConfigOnce.Do(func() {
		key := "InfraGroup"
		if !viper.IsSet(key) {
			infragroup = nil
			return
		}
		viper.UnmarshalKey(key, infragroup)
	})
	return infragroup
}

// NewOSConfig reads OSGroup configuration
func NewOSConfig() *OSGroup {
	osConfigOnce.Do(func() {
		viper.UnmarshalKey("OSGroup", osgroup)
	})
	return osgroup
}

// NewCachePoolConfig reads cache pool layout configuration
func NewCachePoolConfig() *CachePool {
	cachePoolConfigOnce.Do(func() {
		viper.UnmarshalKey("CachePool", cachepool)
	})
	return cachepool
}
