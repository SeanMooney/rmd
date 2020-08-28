package plugins

import (
	"fmt"
	"plugin"

	util "github.com/intel/rmd/utils"
	"github.com/spf13/viper"
)

const (
	symbolName = "Handle"
)

// Load opens file given in path param and tries to load symbol "Handle" implementing ModuleInterface
// Returns error if failed to open file, load symbol or cast interface
func Load(path string) (ModuleInterface, error) {
	// additional verification if given path points to file
	isfile, err := util.IsRegularFile(path)
	if err != nil || !isfile {
		return nil, fmt.Errorf("Invalid plugin path %s", path)
	}

	plg, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open plugin file %s: %s", path, err.Error())
	}
	sym, err := plg.Lookup(symbolName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load symbol %s: %s", symbolName, err.Error())
	}

	result, ok := sym.(ModuleInterface)
	if !ok {
		return nil, fmt.Errorf("Symbol %s in not implementing ModuleInterface", symbolName)
	}

	return result, nil
}

// GetConfig reads configuration section given by name and returns it as a map of string-interface pairs
func GetConfig(name string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	err := viper.UnmarshalKey(name, &result)
	if err != nil {
		// Error is not related to lack of configuration
		return map[string]interface{}{}, fmt.Errorf("plugins.GetConfig(%s) failed: %s", name, err.Error())
	}
	// viper is not informing about lack of section here so additional check needed
	if len(result) == 0 {
		return map[string]interface{}{}, fmt.Errorf("No config for %v found", name)
	}

	return result, nil
}
