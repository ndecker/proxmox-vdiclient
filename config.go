package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func defaultConfig() *Config {
	return &Config{
		ClientConfig: defaultClientConfig(),
	}
}

func defaultConfigFile(program string) string {
	path, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	filename := filepath.Join(path, program, program+".conf")
	return filename

}

// loadConfigFile loads a config file and applies it to a FlagSet
func loadConfigFile(flagSet *flag.FlagSet, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	alreadySetFlags := make(map[string]struct{})
	flagSet.Visit(func(f *flag.Flag) { alreadySetFlags[f.Name] = struct{}{} })

	scan := bufio.NewScanner(f)

	for scan.Scan() {
		line := scan.Text()
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue // comment
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid config line: %s", line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		_, present := alreadySetFlags[key]
		if present {
			continue
		}

		err := flagSet.Set(key, value)
		if err != nil {
			return err
		}
	}

	return nil
}
