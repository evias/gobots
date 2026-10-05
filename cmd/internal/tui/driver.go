package tui

import (
	"strings"

	"github.com/evias/gobots/botfile"
)

// PrintTable uses [PrintTable] to print information about a loaded
// [botfile.Driver] instance.
// Note that this method requires the console to be set in raw mode.
func PrintDriver(driverFile string, driver botfile.Driver) error {
	driverCfg := driver.Config()

	// Build list of commands from driver configuration's commands map.
	commandsMap := driverCfg.Commands
	commands := []string{}
	for c := range commandsMap {
		if len(c) > 0 {
			commands = append(commands, c)
		}
	}

	// Print as a table.
	tableRows := []TableRow{
		{Values: []string{"File:\t", driverFile}},
		{Values: []string{"Name:\t", driver.Name()}},
		{Values: []string{"Address:", driverCfg.Connection.Address()}},
		{Values: []string{"Brand:\t", driverCfg.Brand}},
		{Values: []string{"Repository:", driverCfg.Repository}},
		{Values: []string{"Connection:", driverCfg.Connection.String()}},
		{Values: []string{"Commands:", strings.Join(commands, ", ")}},
	}

	return PrintTable(tableRows)
}
